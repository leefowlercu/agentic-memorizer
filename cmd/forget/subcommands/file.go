package subcommands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/fileops"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/spf13/cobra"
)

var fileDryRun bool

var FileCmd = &cobra.Command{
	Use:   "file <path>...",
	Short: "Move files from memory to forgotten directory",
	Long: "\nMove files or directories from the memory directory to the forgotten directory.\n\n" +
		"Files are moved (not deleted) to ~/.memorizer/.forgotten/ preserving their " +
		"relative path structure. This is a non-destructive operation - files can be " +
		"recovered by using 'remember file' on the forgotten location.\n\n" +
		"The daemon will automatically detect the removal and update the knowledge graph.",
	Example: `  # Forget a single file
  memorizer forget file ~/.memorizer/memory/old-notes.md

  # Forget multiple files
  memorizer forget file doc1.md doc2.md doc3.md

  # Forget a directory recursively
  memorizer forget file ~/.memorizer/memory/archived/

  # Preview what would happen
  memorizer forget file --dry-run large-project/`,
	Args:    cobra.MinimumNArgs(1),
	PreRunE: validateForgetFile,
	RunE:    runForgetFile,
}

func init() {
	FileCmd.Flags().BoolVar(&fileDryRun, "dry-run", false, "Show what would be moved without making changes")
}

func validateForgetFile(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	memoryRoot := cfg.Memory.Root

	// Validate each path exists and is within memory directory
	for _, path := range args {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s; %w", path, err)
		}

		// Check path exists
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", path)
			}
			return fmt.Errorf("cannot access path %s; %w", path, err)
		}

		// Check path is within memory directory
		inMemory, err := fileops.IsInDirectory(absPath, memoryRoot)
		if err != nil {
			return fmt.Errorf("failed to verify path %s; %w", path, err)
		}
		if !inMemory {
			return fmt.Errorf("path is not in memory directory: %s (memory root: %s)", path, memoryRoot)
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runForgetFile(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	memoryRoot := cfg.Memory.Root

	forgottenDir, err := config.GetForgottenDir()
	if err != nil {
		return fmt.Errorf("failed to get forgotten directory; %w", err)
	}

	// Ensure forgotten directory exists
	if !fileDryRun {
		if err := fileops.EnsureDir(forgottenDir); err != nil {
			return fmt.Errorf("failed to create forgotten directory; %w", err)
		}
	}

	// Track results
	var results []forgetFileResult
	successCount := 0
	errorCount := 0

	for _, srcPath := range args {
		absPath, err := filepath.Abs(srcPath)
		if err != nil {
			results = append(results, forgetFileResult{
				path:    srcPath,
				success: false,
				message: fmt.Sprintf("failed to resolve path; %v", err),
			})
			errorCount++
			continue
		}

		// Calculate relative path from memory root
		relPath, err := fileops.RelativePath(memoryRoot, absPath)
		if err != nil {
			results = append(results, forgetFileResult{
				path:    srcPath,
				success: false,
				message: fmt.Sprintf("failed to get relative path; %v", err),
			})
			errorCount++
			continue
		}

		// Calculate destination in forgotten directory
		dstPath := filepath.Join(forgottenDir, relPath)

		if fileDryRun {
			// Determine what the final path would be
			finalPath := dstPath
			if fileops.PathExists(dstPath) {
				resolved, _ := fileops.ResolveConflict(dstPath)
				finalPath = resolved
			}

			results = append(results, forgetFileResult{
				path:    srcPath,
				success: true,
				message: fmt.Sprintf("would move to %s", finalPath),
				dryRun:  true,
			})
			successCount++
			continue
		}

		// Perform the move
		result, err := fileops.Move(absPath, dstPath)
		if err != nil {
			results = append(results, forgetFileResult{
				path:    srcPath,
				success: false,
				message: fmt.Sprintf("move failed; %v", err),
			})
			errorCount++
			continue
		}

		msg := fmt.Sprintf("moved to %s", result.Dst)
		if result.IsDir {
			msg = fmt.Sprintf("moved %d files to %s", result.FileCount, result.Dst)
		}
		if result.Renamed {
			msg += " (renamed due to conflict)"
		}

		results = append(results, forgetFileResult{
			path:    srcPath,
			success: true,
			message: msg,
		})
		successCount++
	}

	// Output results
	return outputForgetFileResults(results, successCount, errorCount, fileDryRun)
}

type forgetFileResult struct {
	path    string
	success bool
	message string
	dryRun  bool
}

func outputForgetFileResults(results []forgetFileResult, successCount, errorCount int, dryRun bool) error {
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
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
			summaryMsg = fmt.Sprintf("complete: %d forgotten, %d failed", successCount, errorCount)
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
