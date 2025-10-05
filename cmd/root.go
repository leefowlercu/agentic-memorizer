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
	memorizerCmd.Flags().String("format", config.DefaultConfig.Output.Format, "Output format (markdown/xml)")
	memorizerCmd.Flags().Bool("wrap-json", config.DefaultConfig.Output.WrapJSON, "Wrap output in SessionStart hook JSON")
	memorizerCmd.Flags().Bool("verbose", config.DefaultConfig.Output.Verbose, "Verbose output")
	memorizerCmd.Flags().Bool("force-analyze", false, "Force re-analysis of all files")
	memorizerCmd.Flags().Bool("no-semantic", false, "Skip semantic analysis")
	memorizerCmd.Flags().String("analyze-file", "", "Analyze specific file")

	viper.BindPFlag("output.format", memorizerCmd.Flags().Lookup("format"))
	viper.BindPFlag("output.wrap_json", memorizerCmd.Flags().Lookup("wrap-json"))
	viper.BindPFlag("output.verbose", memorizerCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("force_analyze", memorizerCmd.Flags().Lookup("force-analyze"))
	viper.BindPFlag("no_semantic", memorizerCmd.Flags().Lookup("no-semantic"))
	viper.BindPFlag("analyze_file", memorizerCmd.Flags().Lookup("analyze-file"))

	memorizerCmd.AddCommand(cmdinit.InitCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("failed to load config; %w", err)
	}

	forceAnalyze := viper.GetBool("force_analyze")
	noSemantic := viper.GetBool("no_semantic")
	analyzeFile := viper.GetString("analyze_file")

	if noSemantic {
		cfg.Analysis.Enable = false
	}

	cacheManager, err := cache.NewManager(cfg.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to create cache manager; %w", err)
	}

	if forceAnalyze {
		if err := cacheManager.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clear cache: %v\n", err)
		}
	}

	metadataExtractor := metadata.NewExtractor()

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

	if analyzeFile != "" {
		return analyzeSpecificFile(analyzeFile, cfg, metadataExtractor, semanticAnalyzer, cacheManager)
	}

	index, err := buildIndex(cfg, metadataExtractor, semanticAnalyzer, cacheManager)
	if err != nil {
		return fmt.Errorf("failed to build index; %w", err)
	}

	formatter := output.NewFormatter(cfg.Output.Verbose, cfg.Output.ShowRecentDays)

	var content string
	switch cfg.Output.Format {
	case "xml":
		content = formatter.FormatXML(index)
	default: // "markdown"
		content = formatter.FormatMarkdown(index)
	}

	if cfg.Output.WrapJSON {
		jsonOutput, err := formatter.WrapJSON(content, index)
		if err != nil {
			return fmt.Errorf("failed to wrap in JSON; %w", err)
		}
		fmt.Println(jsonOutput)
	} else {
		fmt.Print(content)
	}

	return nil
}

func buildIndex(cfg *config.Config, metadataExtractor *metadata.Extractor, semanticAnalyzer *semantic.Analyzer, cacheManager *cache.Manager) (*types.Index, error) {
	index := &types.Index{
		Generated: time.Now(),
		Root:      cfg.MemoryRoot,
		Entries:   []types.IndexEntry{},
		Stats:     types.IndexStats{},
	}

	skipDirs := []string{".cache", ".git"}

	skipFiles := cfg.Analysis.SkipFiles
	if len(skipFiles) == 0 {
		skipFiles = []string{"agentic-memorizer"}
	}

	entries := []types.IndexEntry{}
	var mu sync.Mutex

	err := walker.Walk(cfg.MemoryRoot, skipDirs, skipFiles, func(path string, info os.FileInfo) error {
		relPath, _ := walker.GetRelPath(cfg.MemoryRoot, path)

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

		fileHash, err := cache.HashFile(path)
		if err != nil {
			if cfg.Output.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to hash %s: %v\n", path, err)
			}
			fileHash = ""
		}
		fileMetadata.Hash = fileHash

		var semanticAnalysis *types.SemanticAnalysis
		if semanticAnalyzer != nil && fileHash != "" {
			cached, err := cacheManager.Get(fileHash)
			if err == nil && cached != nil && !cacheManager.IsStale(cached, fileHash) {
				semanticAnalysis = cached.Semantic
				index.Stats.CachedFiles++
			} else {
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

	index.Entries = entries

	index.Stats.TotalFiles = len(entries)
	for _, entry := range entries {
		index.Stats.TotalSize += entry.Metadata.Size
		if entry.Error != nil {
			index.Stats.ErrorFiles++
		}
	}

	return index, nil
}

func analyzeSpecificFile(
	filePath string,
	cfg *config.Config,
	metadataExtractor *metadata.Extractor,
	semanticAnalyzer *semantic.Analyzer,
	cacheManager *cache.Manager,
) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file; %w", err)
	}

	fileMetadata, err := metadataExtractor.Extract(filePath, info)
	if err != nil {
		return fmt.Errorf("failed to extract metadata; %w", err)
	}

	fileHash, _ := cache.HashFile(filePath)
	fileMetadata.Hash = fileHash

	if semanticAnalyzer != nil {
		fmt.Fprintf(os.Stderr, "Analyzing %s...\n\n", filepath.Base(filePath))

		analysis, err := semanticAnalyzer.Analyze(fileMetadata)
		if err != nil {
			return fmt.Errorf("analysis failed; %w", err)
		}

		fmt.Printf("Summary: %s\n\n", analysis.Summary)
		fmt.Printf("Document Type: %s\n\n", analysis.DocumentType)
		fmt.Printf("Key Topics:\n")
		for _, topic := range analysis.KeyTopics {
			fmt.Printf("  - %s\n", topic)
		}
		fmt.Printf("\nTags: %s\n", analysis.Tags)

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
