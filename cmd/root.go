package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	cmdinit "github.com/leefowlercu/agentic-memorizer/cmd/init"
	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/leefowlercu/agentic-memorizer/internal/output"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var memorizerCmd = &cobra.Command{
	Use:   "agentic-memorizer",
	Short: "Agentic Memorizer for Claude Code or Claude Agents",
	Long: "\nA local file 'memorizer' for Claude Code and Claude Agents that provides automatic " +
		"awareness and understanding of files in your memory directory through AI-powered semantic analysis.\n\n" +
		"Agentic Memorizer integrates with Claude Code or Claude Agents via SessionStart hooks to automatically " +
		"index and semantically analyze files in a configured memory directory (default `~/.agentic-memorizer/memory/`). " +
		"Instead of manually adding files to context, Claude Code or Claude Agents automatically receive a " +
		"structured index in their context window showing what files exist, what they contain, and how to access them.",
	PersistentPreRunE: runInit,
	RunE:              runMemorizer,
}

func init() {
	memorizerCmd.Flags().String("format", config.DefaultConfig.Output.Format, "Output format (markdown/json)")
	memorizerCmd.Flags().Bool("verbose", config.DefaultConfig.Output.Verbose, "Verbose output")
	memorizerCmd.Flags().Bool("force-analyze", false, "Force re-analysis of all files")
	memorizerCmd.Flags().Bool("no-semantic", false, "Skip semantic analysis")
	memorizerCmd.Flags().String("analyze-file", "", "Analyze specific file")

	viper.BindPFlag("output.format", memorizerCmd.Flags().Lookup("format"))
	viper.BindPFlag("output.verbose", memorizerCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("force_analyze", memorizerCmd.Flags().Lookup("force-analyze"))
	viper.BindPFlag("no_semantic", memorizerCmd.Flags().Lookup("no-semantic"))
	viper.BindPFlag("analyze_file", memorizerCmd.Flags().Lookup("analyze-file"))

	memorizerCmd.AddCommand(cmdinit.InitCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Only initialize program config if not running init subcommand (which itself initializes user config)
	if cmd.Name() == "init" {
		return nil
	}

	err := config.InitConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize configuration; %w", err)
	}

	return nil
}

func runMemorizer(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	forceAnalyze := viper.GetBool("force_analyze")
	noSemantic := viper.GetBool("no_semantic")
	analyzeFile := viper.GetString("analyze_file")

	if noSemantic {
		cfg.Analysis.Enable = false
	}

	cacheManager, err := cache.NewManager(cfg.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to create cache manager: %w", err)
	}

	if forceAnalyze {
		if err := cacheManager.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clear cache: %v\n", err)
		}
	}

	// Create metadata extractor
	metadataExtractor := metadata.NewExtractor()

	// Create semantic analyzer if enabled
	var semanticAnalyzer *semantic.Analyzer
	if cfg.Analysis.Enable {
		client := semantic.NewClient(
			cfg.Claude.APIKey,
			cfg.Claude.Model,
			cfg.Claude.MaxTokens,
			cfg.Claude.TimeoutSeconds,
		)
		semanticAnalyzer = semantic.NewAnalyzer(
			client,
			cfg.Claude.EnableVision,
			cfg.Analysis.MaxFileSize,
		)
	}

	// If analyzing a specific file
	if analyzeFile != "" {
		return analyzeSpecificFile(analyzeFile, cfg, metadataExtractor, semanticAnalyzer, cacheManager)
	}

	// Build index
	index, err := buildIndex(cfg, metadataExtractor, semanticAnalyzer, cacheManager)
	if err != nil {
		return fmt.Errorf("failed to build index: %w", err)
	}

	// Format output
	formatter := output.NewFormatter(cfg.Output.Verbose, cfg.Output.ShowRecentDays)

	if cfg.Output.Format == "json" {
		// JSON output for Claude Code hooks
		jsonOutput, err := formatter.FormatJSON(index)
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(jsonOutput)
	} else {
		// Markdown output
		markdown := formatter.FormatMarkdown(index)
		fmt.Print(markdown)
	}

	return nil
}

// buildIndex builds the memory index
func buildIndex(cfg *config.Config, metadataExtractor *metadata.Extractor, semanticAnalyzer *semantic.Analyzer, cacheManager *cache.Manager) (*types.Index, error) {
	index := &types.Index{
		Generated: time.Now(),
		Root:      cfg.MemoryRoot,
		Entries:   []types.IndexEntry{},
		Stats:     types.IndexStats{},
	}

	// Skip directories
	skipDirs := []string{".cache", ".git"}

	// Skip files (use config or default)
	skipFiles := cfg.Analysis.SkipFiles
	if len(skipFiles) == 0 {
		skipFiles = []string{"agentic-memorizer"}
	}

	// Walk the file tree
	entries := []types.IndexEntry{}
	var mu sync.Mutex

	err := walker.Walk(cfg.MemoryRoot, skipDirs, skipFiles, func(path string, info os.FileInfo) error {
		// Get relative path
		relPath, _ := walker.GetRelPath(cfg.MemoryRoot, path)

		// Extract metadata
		fileMetadata, err := metadataExtractor.Extract(path, info)
		if err != nil {
			errStr := err.Error()
			mu.Lock()
			entries = append(entries, types.IndexEntry{
				Metadata: *fileMetadata,
				Error:    &errStr,
			})
			mu.Unlock()
			return nil
		}

		fileMetadata.RelPath = relPath

		// Compute file hash
		fileHash, err := cache.HashFile(path)
		if err != nil {
			if cfg.Output.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to hash %s: %v\n", path, err)
			}
			fileHash = ""
		}
		fileMetadata.Hash = fileHash

		// Check cache
		var semanticAnalysis *types.SemanticAnalysis
		if semanticAnalyzer != nil && fileHash != "" {
			cached, err := cacheManager.Get(fileHash)
			if err == nil && cached != nil && !cacheManager.IsStale(cached, fileHash) {
				// Use cached analysis
				semanticAnalysis = cached.Semantic
				index.Stats.CachedFiles++
			} else {
				// Perform semantic analysis
				if cfg.Output.Verbose {
					fmt.Fprintf(os.Stderr, "Analyzing: %s\n", relPath)
				}

				analysis, err := semanticAnalyzer.Analyze(fileMetadata)
				if err != nil {
					if cfg.Output.Verbose {
						fmt.Fprintf(os.Stderr, "Warning: failed to analyze %s: %v\n", path, err)
					}
				} else {
					semanticAnalysis = analysis

					// Cache the analysis
					cachedAnalysis := &types.CachedAnalysis{
						FilePath:   path,
						FileHash:   fileHash,
						AnalyzedAt: time.Now(),
						Metadata:   *fileMetadata,
						Semantic:   semanticAnalysis,
					}
					if err := cacheManager.Set(cachedAnalysis); err != nil {
						if cfg.Output.Verbose {
							fmt.Fprintf(os.Stderr, "Warning: failed to cache analysis for %s: %v\n", path, err)
						}
					}
					index.Stats.AnalyzedFiles++
				}
			}
		}

		// Create index entry
		entry := types.IndexEntry{
			Metadata: *fileMetadata,
			Semantic: semanticAnalysis,
		}

		mu.Lock()
		entries = append(entries, entry)
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Update index
	index.Entries = entries

	// Calculate stats
	index.Stats.TotalFiles = len(entries)
	for _, entry := range entries {
		index.Stats.TotalSize += entry.Metadata.Size
		if entry.Error != nil {
			index.Stats.ErrorFiles++
		}
	}

	return index, nil
}

// analyzeSpecificFile analyzes a specific file and prints the result
func analyzeSpecificFile(
	filePath string,
	cfg *config.Config,
	metadataExtractor *metadata.Extractor,
	semanticAnalyzer *semantic.Analyzer,
	cacheManager *cache.Manager,
) error {
	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Extract metadata
	fileMetadata, err := metadataExtractor.Extract(filePath, info)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Compute hash
	fileHash, _ := cache.HashFile(filePath)
	fileMetadata.Hash = fileHash

	// Perform semantic analysis
	if semanticAnalyzer != nil {
		fmt.Fprintf(os.Stderr, "Analyzing %s...\n\n", filepath.Base(filePath))

		analysis, err := semanticAnalyzer.Analyze(fileMetadata)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		// Print results
		fmt.Printf("Summary: %s\n\n", analysis.Summary)
		fmt.Printf("Document Type: %s\n\n", analysis.DocumentType)
		fmt.Printf("Key Topics:\n")
		for _, topic := range analysis.KeyTopics {
			fmt.Printf("  - %s\n", topic)
		}
		fmt.Printf("\nTags: %s\n", analysis.Tags)

		// Cache the result
		cached := &types.CachedAnalysis{
			FilePath:   filePath,
			FileHash:   fileHash,
			AnalyzedAt: time.Now(),
			Metadata:   *fileMetadata,
			Semantic:   analysis,
		}
		if err := cacheManager.Set(cached); err != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: failed to cache: %v\n", err)
		}
	} else {
		fmt.Println("Semantic analysis disabled")
	}

	return nil
}

func Execute() error {
	memorizerCmd.SilenceErrors = true
	memorizerCmd.SilenceUsage = true

	err := memorizerCmd.Execute()

	if err != nil {
		cmd, _, _ := memorizerCmd.Find(os.Args[1:])
		if cmd == nil {
			cmd = memorizerCmd
		}

		fmt.Printf("Error: %v\n", err)
		if !cmd.SilenceUsage {
			fmt.Printf("\n")
			cmd.SetOut(os.Stdout)
			cmd.Usage()
		}

		return err
	}

	return nil
}
