// Package remember implements the remember command for registering directories.
package remember

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/cmdutil"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/daemonclient"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// Flag variables for the remember command.
var (
	rememberAddSkipExt     []string
	rememberSetSkipExt     []string
	rememberAddSkipDir     []string
	rememberSetSkipDir     []string
	rememberAddSkipFiles   []string
	rememberSetSkipFiles   []string
	rememberAddIncludeExt  []string
	rememberAddIncludeDir  []string
	rememberAddIncludeFile []string
	rememberSkipHidden     bool
	rememberUseVision      *bool
)

// useVisionFlag is a custom flag type to track if --use-vision was explicitly set.
var useVisionFlag string

// RememberCmd is the remember command for registering directories.
var RememberCmd = &cobra.Command{
	Use:   "remember <path>",
	Short: "Register a directory for tracking",
	Long: "Register a directory for tracking by the memorizer daemon.\n\n" +
		"When a directory is remembered, its files will be analyzed for metadata, " +
		"semantic content, and embeddings. The daemon will watch for changes and " +
		"keep the knowledge graph up to date.\n\n" +
		"Skip and include rules can be specified to control which files are processed. " +
		"Use --add-* flags to extend default rules, or --set-* flags to replace them entirely. " +
		"Include rules override corresponding skip rules for the same items.",
	Example: `  # Remember a project directory with default settings
  memorizer remember ~/projects/myapp

  # Remember with additional extensions to skip (additive to defaults)
  memorizer remember ~/documents --add-skip-ext=.log,.tmp

  # Remember with replaced skip rules (ignores defaults)
  memorizer remember ~/special --set-skip-ext=.bak

  # Remember with include overrides (include .env even though hidden)
  memorizer remember ~/config --add-include-file=.env,.envrc

  # Remember without vision API processing for images
  memorizer remember ~/large-images --use-vision=false`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateRemember,
	RunE:    runRemember,
}

func init() {
	// Skip flags (additive)
	RememberCmd.Flags().StringSliceVar(&rememberAddSkipExt, "add-skip-ext", nil,
		"Add extensions to skip (additive to defaults)")
	RememberCmd.Flags().StringSliceVar(&rememberAddSkipDir, "add-skip-dir", nil,
		"Add directories to skip (additive to defaults)")
	RememberCmd.Flags().StringSliceVar(&rememberAddSkipFiles, "add-skip-file", nil,
		"Add files to skip (additive to defaults)")

	// Skip flags (replace)
	RememberCmd.Flags().StringSliceVar(&rememberSetSkipExt, "set-skip-ext", nil,
		"Set extensions to skip (replaces defaults)")
	RememberCmd.Flags().StringSliceVar(&rememberSetSkipDir, "set-skip-dir", nil,
		"Set directories to skip (replaces defaults)")
	RememberCmd.Flags().StringSliceVar(&rememberSetSkipFiles, "set-skip-file", nil,
		"Set files to skip (replaces defaults)")

	// Include flags (override skip)
	RememberCmd.Flags().StringSliceVar(&rememberAddIncludeExt, "add-include-ext", nil,
		"Add extensions to include (overrides skip)")
	RememberCmd.Flags().StringSliceVar(&rememberAddIncludeDir, "add-include-dir", nil,
		"Add directories to include (overrides skip)")
	RememberCmd.Flags().StringSliceVar(&rememberAddIncludeFile, "add-include-file", nil,
		"Add files to include (overrides skip)")

	// Hidden file handling
	RememberCmd.Flags().BoolVar(&rememberSkipHidden, "skip-hidden", true,
		"Skip hidden files and directories")

	// Vision API
	RememberCmd.Flags().StringVar(&useVisionFlag, "use-vision", "",
		"Enable/disable vision API for images/PDFs (true/false)")
}

func validateRemember(cmd *cobra.Command, args []string) error {
	absPath, err := cmdutil.ResolvePath(args[0])
	if err != nil {
		return fmt.Errorf("failed to resolve path; %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", absPath)
		}
		return fmt.Errorf("failed to access path; %w", err)
	}

	// Check if path is a directory
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Parse --use-vision flag if provided
	if useVisionFlag != "" {
		switch strings.ToLower(useVisionFlag) {
		case "true", "1", "yes":
			v := true
			rememberUseVision = &v
		case "false", "0", "no":
			v := false
			rememberUseVision = &v
		default:
			return fmt.Errorf("invalid value for --use-vision: %s (expected true/false)", useVisionFlag)
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRemember(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := cmd.OutOrStdout()
	quiet := isQuiet(cmd)
	absPath, err := cmdutil.ResolvePath(args[0])
	if err != nil {
		return fmt.Errorf("failed to resolve path; %w", err)
	}

	patch := buildConfigPatch(cmd)
	client, err := daemonclient.NewFromConfig(config.Get())
	if err != nil {
		return fmt.Errorf("failed to initialize daemon client; %w", err)
	}

	result, err := client.Remember(ctx, daemon.RememberRequest{
		Path:  absPath,
		Patch: patch,
	})
	if err != nil {
		return fmt.Errorf("remember request failed; %w", err)
	}

	if quiet {
		return nil
	}

	switch result.Status {
	case daemon.RememberStatusUpdated:
		fmt.Fprintf(out, "Updated: %s\n", absPath)
	default:
		fmt.Fprintf(out, "Remembered: %s\n", absPath)
	}

	return nil
}

func isQuiet(cmd *cobra.Command) bool {
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return false
	}
	return quiet
}

func buildConfigPatch(cmd *cobra.Command) *registry.PathConfigPatch {
	patch := &registry.PathConfigPatch{}

	if cmd.Flags().Changed("skip-hidden") {
		value := rememberSkipHidden
		patch.SkipHidden = &value
	}

	if cmd.Flags().Changed("use-vision") {
		patch.UseVision = rememberUseVision
	}

	if cmd.Flags().Changed("set-skip-ext") {
		patch.SetSkipExtensions = rememberSetSkipExt
	}
	if cmd.Flags().Changed("add-skip-ext") {
		patch.AddSkipExtensions = rememberAddSkipExt
	}

	if cmd.Flags().Changed("set-skip-dir") {
		patch.SetSkipDirectories = rememberSetSkipDir
	}
	if cmd.Flags().Changed("add-skip-dir") {
		patch.AddSkipDirectories = rememberAddSkipDir
	}

	if cmd.Flags().Changed("set-skip-file") {
		patch.SetSkipFiles = rememberSetSkipFiles
	}
	if cmd.Flags().Changed("add-skip-file") {
		patch.AddSkipFiles = rememberAddSkipFiles
	}

	if cmd.Flags().Changed("add-include-ext") {
		patch.AddIncludeExtensions = rememberAddIncludeExt
	}
	if cmd.Flags().Changed("add-include-dir") {
		patch.AddIncludeDirectories = rememberAddIncludeDir
	}
	if cmd.Flags().Changed("add-include-file") {
		patch.AddIncludeFiles = rememberAddIncludeFile
	}

	if patch.IsEmpty() {
		return nil
	}
	return patch
}
