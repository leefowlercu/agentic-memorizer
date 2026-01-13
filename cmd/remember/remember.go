// Package remember implements the remember command for registering directories.
package remember

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/spf13/cobra"
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
	rememberIncludeHidden  bool
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
	RememberCmd.Flags().BoolVar(&rememberIncludeHidden, "include-hidden", false,
		"Include hidden files even when skip-hidden is true")

	// Vision API
	RememberCmd.Flags().StringVar(&useVisionFlag, "use-vision", "",
		"Enable/disable vision API for images/PDFs (true/false)")
}

func validateRemember(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory; %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
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
	path := args[0]

	// Expand and resolve path
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	absPath, _ := filepath.Abs(path)
	absPath = filepath.Clean(absPath)

	// Open registry
	registryPath := config.ExpandPath(config.Get().Daemon.RegistryPath)
	reg, err := registry.Open(ctx, registryPath)
	if err != nil {
		return fmt.Errorf("failed to open registry; %w", err)
	}
	defer reg.Close()

	// Build path config
	pathConfig := buildPathConfig(cmd)

	// Add path to registry
	err = reg.AddPath(ctx, absPath, pathConfig)
	if err != nil {
		if err == registry.ErrPathExists {
			return fmt.Errorf("path is already remembered: %s", absPath)
		}
		return fmt.Errorf("failed to remember path; %w", err)
	}

	fmt.Printf("Remembered: %s\n", absPath)
	return nil
}

// buildPathConfig constructs a PathConfig from command flags.
func buildPathConfig(cmd *cobra.Command) *registry.PathConfig {
	cfg := &registry.PathConfig{
		SkipHidden:    rememberSkipHidden,
		IncludeHidden: rememberIncludeHidden,
		UseVision:     rememberUseVision,
	}

	// Handle skip extensions
	if cmd.Flags().Changed("set-skip-ext") {
		cfg.SkipExtensions = normalizeExtensions(rememberSetSkipExt)
	} else if cmd.Flags().Changed("add-skip-ext") {
		cfg.SkipExtensions = normalizeExtensions(rememberAddSkipExt)
	}

	// Handle skip directories
	if cmd.Flags().Changed("set-skip-dir") {
		cfg.SkipDirectories = rememberSetSkipDir
	} else if cmd.Flags().Changed("add-skip-dir") {
		cfg.SkipDirectories = rememberAddSkipDir
	}

	// Handle skip files
	if cmd.Flags().Changed("set-skip-file") {
		cfg.SkipFiles = rememberSetSkipFiles
	} else if cmd.Flags().Changed("add-skip-file") {
		cfg.SkipFiles = rememberAddSkipFiles
	}

	// Handle include flags
	if cmd.Flags().Changed("add-include-ext") {
		cfg.IncludeExtensions = normalizeExtensions(rememberAddIncludeExt)
	}
	if cmd.Flags().Changed("add-include-dir") {
		cfg.IncludeDirectories = rememberAddIncludeDir
	}
	if cmd.Flags().Changed("add-include-file") {
		cfg.IncludeFiles = rememberAddIncludeFile
	}

	return cfg
}

// normalizeExtensions ensures all extensions start with a dot.
func normalizeExtensions(exts []string) []string {
	result := make([]string, len(exts))
	for i, ext := range exts {
		ext = strings.TrimSpace(ext)
		if ext != "" && !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		result[i] = ext
	}
	return result
}
