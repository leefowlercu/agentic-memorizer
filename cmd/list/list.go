// Package list implements the list command for displaying remembered directories.
package list

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/spf13/cobra"
)

// Flag variables for the list command.
var (
	listVerbose bool
)

// ListCmd is the list command for displaying remembered directories.
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all remembered directories",
	Long: "List all directories that are currently being tracked by memorizer.\n\n" +
		"Use --verbose to display detailed configuration information for each directory, " +
		"including skip/include rules and other settings.",
	Example: `  # List remembered directories
  memorizer list

  # List with detailed configuration
  memorizer list --verbose`,
	Args:    cobra.NoArgs,
	PreRunE: validateList,
	RunE:    runList,
}

func init() {
	ListCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false,
		"Show detailed configuration for each directory")
}

func validateList(cmd *cobra.Command, args []string) error {
	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	// Open registry
	registryPath := config.GetPath("database.registry_path")
	reg, err := registry.Open(ctx, registryPath)
	if err != nil {
		return fmt.Errorf("failed to open registry; %w", err)
	}
	defer reg.Close()

	// Get all remembered paths
	paths, err := reg.ListPaths(ctx)
	if err != nil {
		return fmt.Errorf("failed to list paths; %w", err)
	}

	if len(paths) == 0 {
		fmt.Fprintln(out, "No directories remembered.")
		fmt.Fprintln(out, "\nUse 'memorizer remember <path>' to start tracking a directory.")
		return nil
	}

	fmt.Fprintf(out, "Remembered directories (%d):\n\n", len(paths))

	for _, p := range paths {
		if listVerbose {
			printVerbosePath(ctx, out, reg, &p)
		} else {
			printSimplePath(out, &p)
		}
	}

	return nil
}

func printSimplePath(out io.Writer, p *registry.RememberedPath) {
	fmt.Fprintf(out, "  %s\n", p.Path)
}

func printVerbosePath(ctx context.Context, out io.Writer, reg *registry.SQLiteRegistry, p *registry.RememberedPath) {
	fmt.Fprintf(out, "  Path: %s\n", p.Path)

	// Print timestamps
	fmt.Fprintf(out, "    Added: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	if p.LastWalkAt != nil {
		fmt.Fprintf(out, "    Last Walk: %s\n", p.LastWalkAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Fprintf(out, "    Last Walk: never\n")
	}

	// Count files
	fileStates, err := reg.ListFileStates(ctx, p.Path)
	if err == nil {
		fmt.Fprintf(out, "    Files Tracked: %d\n", len(fileStates))
	}

	// Print configuration if present
	if p.Config != nil {
		printConfig(out, p.Config)
	}

	fmt.Fprintln(out)
}

func printConfig(out io.Writer, cfg *registry.PathConfig) {
	fmt.Fprintf(out, "    Configuration:\n")

	// Skip rules
	if len(cfg.SkipExtensions) > 0 {
		fmt.Fprintf(out, "      Skip Extensions: %s\n", strings.Join(cfg.SkipExtensions, ", "))
	}
	if len(cfg.SkipDirectories) > 0 {
		fmt.Fprintf(out, "      Skip Directories: %s\n", strings.Join(cfg.SkipDirectories, ", "))
	}
	if len(cfg.SkipFiles) > 0 {
		fmt.Fprintf(out, "      Skip Files: %s\n", strings.Join(cfg.SkipFiles, ", "))
	}
	fmt.Fprintf(out, "      Skip Hidden: %t\n", cfg.SkipHidden)

	// Include rules
	if len(cfg.IncludeExtensions) > 0 {
		fmt.Fprintf(out, "      Include Extensions: %s\n", strings.Join(cfg.IncludeExtensions, ", "))
	}
	if len(cfg.IncludeDirectories) > 0 {
		fmt.Fprintf(out, "      Include Directories: %s\n", strings.Join(cfg.IncludeDirectories, ", "))
	}
	if len(cfg.IncludeFiles) > 0 {
		fmt.Fprintf(out, "      Include Files: %s\n", strings.Join(cfg.IncludeFiles, ", "))
	}
	if cfg.IncludeHidden {
		fmt.Fprintf(out, "      Include Hidden: %t\n", cfg.IncludeHidden)
	}

	// Vision API
	if cfg.UseVision != nil {
		fmt.Fprintf(out, "      Use Vision: %t\n", *cfg.UseVision)
	}
}

// FormatConfigJSON returns the configuration as a JSON string for debugging.
func FormatConfigJSON(cfg *registry.PathConfig) string {
	if cfg == nil {
		return "{}"
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}
