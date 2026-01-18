// Package list implements the list command for displaying remembered directories.
package list

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/cmdutil"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
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
	registryPath, err := cmdutil.ResolvePath(config.Get().Daemon.RegistryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve registry path; %w", err)
	}
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

	// Get health status for all paths
	statuses, err := reg.CheckPathHealth(ctx)
	if err != nil {
		return fmt.Errorf("failed to check path health; %w", err)
	}

	// Build status lookup map
	statusMap := make(map[string]string)
	for _, s := range statuses {
		statusMap[s.Path] = s.Status
	}

	fmt.Fprintf(out, "Remembered directories (%d):\n\n", len(paths))

	if listVerbose {
		for _, p := range paths {
			status := statusMap[p.Path]
			printVerbosePath(ctx, out, reg, &p, status)
		}
	} else {
		// Print table header
		fmt.Fprintf(out, "%-40s %-10s %-8s %s\n", "PATH", "STATUS", "FILES", "LAST WALK")
		fmt.Fprintf(out, "%-40s %-10s %-8s %s\n", strings.Repeat("-", 40), strings.Repeat("-", 10), strings.Repeat("-", 8), strings.Repeat("-", 19))

		for _, p := range paths {
			status := statusMap[p.Path]
			printTableRow(ctx, out, reg, &p, status)
		}
	}

	return nil
}

func printTableRow(ctx context.Context, out io.Writer, reg *registry.SQLiteRegistry, p *registry.RememberedPath, status string) {
	// Truncate path if too long
	path := p.Path
	if len(path) > 40 {
		path = "..." + path[len(path)-37:]
	}

	// Get file count (show "-" if path is inaccessible)
	filesStr := "-"
	if status == registry.PathStatusOK {
		fileStates, err := reg.ListFileStates(ctx, p.Path)
		if err == nil {
			filesStr = fmt.Sprintf("%d", len(fileStates))
		}
	}

	// Format last walk time (show "-" if never walked or inaccessible)
	lastWalkStr := "-"
	if status == registry.PathStatusOK && p.LastWalkAt != nil {
		lastWalkStr = p.LastWalkAt.Format("2006-01-02 15:04:05")
	}

	fmt.Fprintf(out, "%-40s %-10s %-8s %s\n", path, status, filesStr, lastWalkStr)
}

func printVerbosePath(ctx context.Context, out io.Writer, reg *registry.SQLiteRegistry, p *registry.RememberedPath, status string) {
	fmt.Fprintf(out, "  Path: %s\n", p.Path)
	fmt.Fprintf(out, "    Status: %s\n", status)

	// Print timestamps
	fmt.Fprintf(out, "    Added: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	if p.LastWalkAt != nil {
		fmt.Fprintf(out, "    Last Walk: %s\n", p.LastWalkAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Fprintf(out, "    Last Walk: never\n")
	}

	// Count files (only if path is accessible)
	if status == registry.PathStatusOK {
		fileStates, err := reg.ListFileStates(ctx, p.Path)
		if err == nil {
			fmt.Fprintf(out, "    Files Tracked: %d\n", len(fileStates))
		}
	} else {
		fmt.Fprintf(out, "    Files Tracked: -\n")
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
