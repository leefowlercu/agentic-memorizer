package subcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/fileops"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/spf13/cobra"
)

var (
	fileDir    string
	fileForce  bool
	fileDryRun bool
)

// largeFileBatchThreshold is the number of files that triggers a warning
const largeFileBatchThreshold = 100

var FileCmd = &cobra.Command{
	Use:   "file <path>...",
	Short: "Copy files into memory directory",
	Long: "\nCopy files or directories into the memory directory.\n\n" +
		"Files are copied (not moved) to preserve the original. If a file with the " +
		"same name already exists in the destination, it is automatically renamed " +
		"with a -N suffix (e.g., file.md → file-1.md).\n\n" +
		"The daemon will automatically detect and index new files.",
	Example: `  # Remember a single file
  memorizer remember file ~/notes/project-ideas.md

  # Remember multiple files
  memorizer remember file doc1.md doc2.md doc3.md

  # Remember a directory recursively
  memorizer remember file ~/project/docs/

  # Remember into a subdirectory
  memorizer remember file --dir work/notes report.md

  # Force overwrite existing files
  memorizer remember file --force existing-file.md

  # Preview what would happen
  memorizer remember file --dry-run largedir/`,
	Args:    cobra.MinimumNArgs(1),
	PreRunE: validateFile,
	RunE:    runFile,
}

func init() {
	FileCmd.Flags().StringVar(&fileDir, "dir", "", "Subdirectory within memory root to copy files into")
	FileCmd.Flags().BoolVar(&fileForce, "force", false, "Overwrite existing files and skip confirmation for large batches")
	FileCmd.Flags().BoolVar(&fileDryRun, "dry-run", false, "Show what would be copied without making changes")
}

func validateFile(cmd *cobra.Command, args []string) error {
	// Validate each path exists and is readable
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", path)
			}
			return fmt.Errorf("cannot access path %s; %w", path, err)
		}

		// Check if readable
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("cannot read directory %s; %w", path, err)
			}
			_ = entries // Just checking readability
		} else {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("cannot read file %s; %w", path, err)
			}
			file.Close()
		}
	}

	// Validate --dir if provided
	if fileDir != "" {
		if err := fileops.ValidateSubdirectory(fileDir); err != nil {
			return fmt.Errorf("invalid --dir value; %w", err)
		}
	}

	// Count total files for large batch warning
	if !fileForce {
		totalFiles := 0
		for _, path := range args {
			count, err := countFilesInPath(path)
			if err != nil {
				return fmt.Errorf("failed to count files in %s; %w", path, err)
			}
			totalFiles += count
		}

		if totalFiles > largeFileBatchThreshold {
			return fmt.Errorf("large batch detected (%d files); use --force to proceed or split into smaller batches", totalFiles)
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runFile(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	memoryRoot := cfg.Memory.Root
	destDir := memoryRoot
	if fileDir != "" {
		destDir = filepath.Join(memoryRoot, fileDir)
	}

	// Ensure destination directory exists
	if !fileDryRun {
		if err := fileops.EnsureDir(destDir); err != nil {
			return fmt.Errorf("failed to create destination directory; %w", err)
		}
	}

	// Track results
	var results []fileResult
	var warnings []string
	successCount := 0
	errorCount := 0

	// Check for unsupported file types
	extractor := metadata.NewExtractor()

	for _, srcPath := range args {
		absPath, err := filepath.Abs(srcPath)
		if err != nil {
			results = append(results, fileResult{
				path:    srcPath,
				success: false,
				message: fmt.Sprintf("failed to resolve path; %v", err),
			})
			errorCount++
			continue
		}

		// Check if source is already in memory directory
		inMemory, err := fileops.IsInDirectory(absPath, memoryRoot)
		if err != nil {
			results = append(results, fileResult{
				path:    srcPath,
				success: false,
				message: fmt.Sprintf("failed to check path; %v", err),
			})
			errorCount++
			continue
		}
		if inMemory {
			results = append(results, fileResult{
				path:    srcPath,
				success: false,
				message: "file is already in memory directory",
			})
			errorCount++
			continue
		}

		// Determine destination path
		var dstPath string
		info, _ := os.Stat(absPath)
		if info.IsDir() {
			dstPath = filepath.Join(destDir, filepath.Base(absPath))
		} else {
			dstPath = filepath.Join(destDir, filepath.Base(absPath))

			// Warn if file type has no semantic handler
			ext := strings.ToLower(filepath.Ext(absPath))
			if ext != "" && !hasSemanticHandler(extractor, ext) {
				warnings = append(warnings, fmt.Sprintf("file type %s may not be fully analyzed: %s", ext, filepath.Base(absPath)))
			}
		}

		if fileDryRun {
			// Determine what the final path would be
			finalPath := dstPath
			if !fileForce && fileops.PathExists(dstPath) {
				resolved, _ := fileops.ResolveConflict(dstPath)
				finalPath = resolved
			}

			results = append(results, fileResult{
				path:    srcPath,
				success: true,
				message: fmt.Sprintf("would copy to %s", finalPath),
				dryRun:  true,
			})
			successCount++
			continue
		}

		// Perform the copy
		result, err := fileops.Copy(absPath, dstPath, fileForce)
		if err != nil {
			results = append(results, fileResult{
				path:    srcPath,
				success: false,
				message: fmt.Sprintf("copy failed; %v", err),
			})
			errorCount++
			continue
		}

		msg := fmt.Sprintf("copied to %s", result.Dst)
		if result.IsDir {
			msg = fmt.Sprintf("copied %d files to %s", result.FileCount, result.Dst)
		}
		if result.Renamed {
			msg += " (renamed due to conflict)"
		}

		results = append(results, fileResult{
			path:    srcPath,
			success: true,
			message: msg,
		})
		successCount++
	}

	// Output results
	return outputFileResults(results, warnings, successCount, errorCount, fileDryRun)
}

type fileResult struct {
	path    string
	success bool
	message string
	dryRun  bool
}

func outputFileResults(results []fileResult, warnings []string, successCount, errorCount int, dryRun bool) error {
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}

	// Output warnings first
	for _, warning := range warnings {
		status := format.NewStatus(format.StatusWarning, warning)
		output, _ := formatter.Format(status)
		fmt.Println(output)
	}

	// Output individual results
	for _, result := range results {
		var severity format.StatusSeverity
		if result.success {
			if result.dryRun {
				severity = format.StatusInfo
			} else {
				severity = format.StatusSuccess
			}
		} else {
			severity = format.StatusError
		}

		msg := fmt.Sprintf("%s: %s", result.path, result.message)
		status := format.NewStatus(severity, msg)
		output, _ := formatter.Format(status)
		fmt.Println(output)
	}

	// Output summary
	if len(results) > 1 {
		var summaryMsg string
		if dryRun {
			summaryMsg = fmt.Sprintf("dry run complete: %d would succeed, %d would fail", successCount, errorCount)
		} else {
			summaryMsg = fmt.Sprintf("complete: %d succeeded, %d failed", successCount, errorCount)
		}

		var severity format.StatusSeverity
		if errorCount > 0 {
			severity = format.StatusWarning
		} else {
			severity = format.StatusSuccess
		}

		status := format.NewStatus(severity, summaryMsg)
		output, _ := formatter.Format(status)
		fmt.Println(output)
	}

	// Return error if any operations failed
	if errorCount > 0 {
		return fmt.Errorf("%d operation(s) failed", errorCount)
	}

	return nil
}

// countFilesInPath counts files in a path (1 for file, N for directory)
func countFilesInPath(path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	if !info.IsDir() {
		return 1, nil
	}

	count := 0
	err = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})

	return count, err
}

// hasSemanticHandler checks if there's a metadata handler for the extension
func hasSemanticHandler(extractor *metadata.Extractor, ext string) bool {
	// These extensions have handlers in the metadata package
	supportedExts := map[string]bool{
		".md":    true,
		".docx":  true,
		".pptx":  true,
		".pdf":   true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".webp":  true,
		".bmp":   true,
		".svg":   true,
		".vtt":   true,
		".srt":   true,
		".json":  true,
		".yaml":  true,
		".yml":   true,
		".go":    true,
		".py":    true,
		".js":    true,
		".ts":    true,
		".java":  true,
		".c":     true,
		".cpp":   true,
		".rs":    true,
		".rb":    true,
		".php":   true,
		".sh":    true,
		".bash":  true,
		".html":  true,
		".css":   true,
		".tsx":   true,
		".jsx":   true,
		".swift": true,
		".kt":    true,
		".scala": true,
		".r":     true,
		".sql":   true,
		".lua":   true,
		".pl":    true,
		".ex":    true,
		".exs":   true,
		".hs":    true,
		".ml":    true,
		".clj":   true,
		".cs":    true,
		".fs":    true,
		".txt":   true,
	}

	// The extractor is passed in to avoid creating a new one each time
	// but we use the known supported extensions for the check
	_ = extractor
	return supportedExts[ext]
}
