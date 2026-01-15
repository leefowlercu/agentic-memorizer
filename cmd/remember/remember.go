// Package remember implements the remember command for registering directories.
package remember

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// Check if path already exists
	existingPath, err := reg.GetPath(ctx, absPath)
	if err == nil && existingPath != nil {
		// Path exists - check if modification flags are provided
		if hasModificationFlags(cmd) {
			return handleExistingPath(ctx, cmd, reg, absPath, existingPath)
		}
		return fmt.Errorf("path is already remembered: %s\nUse modification flags (--add-*, --set-*, --skip-hidden) to update configuration", absPath)
	}

	// Build path config for new path
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

// hasModificationFlags returns true if any config modification flags are set.
func hasModificationFlags(cmd *cobra.Command) bool {
	modFlags := []string{
		"add-skip-ext", "set-skip-ext",
		"add-skip-dir", "set-skip-dir",
		"add-skip-file", "set-skip-file",
		"add-include-ext", "add-include-dir", "add-include-file",
		"skip-hidden", "use-vision",
	}
	for _, flag := range modFlags {
		if cmd.Flags().Changed(flag) {
			return true
		}
	}
	return false
}

// handleExistingPath updates the configuration for an already-remembered path.
func handleExistingPath(ctx context.Context, cmd *cobra.Command, reg *registry.SQLiteRegistry, absPath string, existing *registry.RememberedPath) error {
	// Build updated config
	newConfig := buildUpdatedConfig(cmd, existing.Config)

	// Update the config in registry
	err := reg.UpdatePathConfig(ctx, absPath, newConfig)
	if err != nil {
		return fmt.Errorf("failed to update path config; %w", err)
	}

	fmt.Printf("Updated: %s\n", absPath)

	// Trigger re-walk via daemon API
	if err := requestReWalk(); err != nil {
		fmt.Printf("Warning: could not trigger re-walk: %v\n", err)
		fmt.Println("The daemon may need to be restarted for changes to take effect.")
	} else {
		fmt.Println("Re-walk triggered successfully.")
	}

	return nil
}

// buildUpdatedConfig creates a new PathConfig by merging existing config with flag overrides.
func buildUpdatedConfig(cmd *cobra.Command, existing *registry.PathConfig) *registry.PathConfig {
	// Start with a clone of existing config
	cfg := existing.Clone()
	if cfg == nil {
		cfg = &registry.PathConfig{}
	}

	// Override skip-hidden if explicitly set
	if cmd.Flags().Changed("skip-hidden") {
		cfg.SkipHidden = rememberSkipHidden
	}

	// Handle skip extensions
	if cmd.Flags().Changed("set-skip-ext") {
		cfg.SkipExtensions = normalizeExtensions(rememberSetSkipExt)
	} else if cmd.Flags().Changed("add-skip-ext") {
		cfg.SkipExtensions = mergeUnique(cfg.SkipExtensions, normalizeExtensions(rememberAddSkipExt))
	}

	// Handle skip directories
	if cmd.Flags().Changed("set-skip-dir") {
		cfg.SkipDirectories = rememberSetSkipDir
	} else if cmd.Flags().Changed("add-skip-dir") {
		cfg.SkipDirectories = mergeUnique(cfg.SkipDirectories, rememberAddSkipDir)
	}

	// Handle skip files
	if cmd.Flags().Changed("set-skip-file") {
		cfg.SkipFiles = rememberSetSkipFiles
	} else if cmd.Flags().Changed("add-skip-file") {
		cfg.SkipFiles = mergeUnique(cfg.SkipFiles, rememberAddSkipFiles)
	}

	// Handle include extensions
	if cmd.Flags().Changed("add-include-ext") {
		cfg.IncludeExtensions = mergeUnique(cfg.IncludeExtensions, normalizeExtensions(rememberAddIncludeExt))
	}

	// Handle include directories
	if cmd.Flags().Changed("add-include-dir") {
		cfg.IncludeDirectories = mergeUnique(cfg.IncludeDirectories, rememberAddIncludeDir)
	}

	// Handle include files
	if cmd.Flags().Changed("add-include-file") {
		cfg.IncludeFiles = mergeUnique(cfg.IncludeFiles, rememberAddIncludeFile)
	}

	// Handle use-vision
	if rememberUseVision != nil {
		cfg.UseVision = rememberUseVision
	}

	return cfg
}

// requestReWalk triggers an incremental re-walk via the daemon HTTP API.
func requestReWalk() error {
	cfg := config.Get()
	url := fmt.Sprintf("http://%s:%d/rebuild", cfg.Daemon.HTTPBind, cfg.Daemon.HTTPPort)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Error != "" {
			return fmt.Errorf("rebuild failed: %s", result.Error)
		}
		return fmt.Errorf("rebuild failed with status %d", resp.StatusCode)
	}

	return nil
}

// buildPathConfig constructs a PathConfig from command flags merged with defaults.
func buildPathConfig(cmd *cobra.Command) *registry.PathConfig {
	defaults := config.Get().Defaults

	cfg := &registry.PathConfig{
		SkipHidden: defaults.Skip.Hidden,
		UseVision:  rememberUseVision,
	}

	// Override skip-hidden if explicitly set
	if cmd.Flags().Changed("skip-hidden") {
		cfg.SkipHidden = rememberSkipHidden
	}

	// Handle skip extensions: set replaces, add extends defaults
	if cmd.Flags().Changed("set-skip-ext") {
		cfg.SkipExtensions = normalizeExtensions(rememberSetSkipExt)
	} else if cmd.Flags().Changed("add-skip-ext") {
		cfg.SkipExtensions = mergeUnique(defaults.Skip.Extensions, normalizeExtensions(rememberAddSkipExt))
	} else {
		cfg.SkipExtensions = defaults.Skip.Extensions
	}

	// Handle skip directories: set replaces, add extends defaults
	if cmd.Flags().Changed("set-skip-dir") {
		cfg.SkipDirectories = rememberSetSkipDir
	} else if cmd.Flags().Changed("add-skip-dir") {
		cfg.SkipDirectories = mergeUnique(defaults.Skip.Directories, rememberAddSkipDir)
	} else {
		cfg.SkipDirectories = defaults.Skip.Directories
	}

	// Handle skip files: set replaces, add extends defaults
	if cmd.Flags().Changed("set-skip-file") {
		cfg.SkipFiles = rememberSetSkipFiles
	} else if cmd.Flags().Changed("add-skip-file") {
		cfg.SkipFiles = mergeUnique(defaults.Skip.Files, rememberAddSkipFiles)
	} else {
		cfg.SkipFiles = defaults.Skip.Files
	}

	// Handle include extensions: add extends defaults
	if cmd.Flags().Changed("add-include-ext") {
		cfg.IncludeExtensions = mergeUnique(defaults.Include.Extensions, normalizeExtensions(rememberAddIncludeExt))
	} else {
		cfg.IncludeExtensions = defaults.Include.Extensions
	}

	// Handle include directories: add extends defaults
	if cmd.Flags().Changed("add-include-dir") {
		cfg.IncludeDirectories = mergeUnique(defaults.Include.Directories, rememberAddIncludeDir)
	} else {
		cfg.IncludeDirectories = defaults.Include.Directories
	}

	// Handle include files: add extends defaults
	if cmd.Flags().Changed("add-include-file") {
		cfg.IncludeFiles = mergeUnique(defaults.Include.Files, rememberAddIncludeFile)
	} else {
		cfg.IncludeFiles = defaults.Include.Files
	}

	return cfg
}

// mergeUnique combines two slices, removing duplicates while preserving order.
func mergeUnique(base, additions []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(base)+len(additions))
	for _, v := range base {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	for _, v := range additions {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
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
