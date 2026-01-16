package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	configcmd "github.com/leefowlercu/agentic-memorizer/cmd/config"
	"github.com/leefowlercu/agentic-memorizer/cmd/daemon"
	"github.com/leefowlercu/agentic-memorizer/cmd/forget"
	initcmd "github.com/leefowlercu/agentic-memorizer/cmd/initialize"
	"github.com/leefowlercu/agentic-memorizer/cmd/integrations"
	"github.com/leefowlercu/agentic-memorizer/cmd/list"
	"github.com/leefowlercu/agentic-memorizer/cmd/providers"
	"github.com/leefowlercu/agentic-memorizer/cmd/read"
	"github.com/leefowlercu/agentic-memorizer/cmd/remember"
	"github.com/leefowlercu/agentic-memorizer/cmd/version"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/logging"
)

// logManager is the global logging manager, created in init() and upgraded after config loads
var logManager *logging.Manager

// Quiet suppresses non-error output when true
var Quiet bool

var memorizerCmd = &cobra.Command{
	Use:   "memorizer",
	Short: "A Knowledge Graph Based Memorization Tool for AI Agents",
	Long: "Agentic Memorizer provides automatic awareness and analysis capabilities for files contained in registered directories.\n\n" +
		"A background daemon monitors registered directories and all their subdirectories for file additions, updates, moves, and deletions. " +
		"When changes are detected, the tool performs metadata extraction, semantic analysis, and embeddings generation for the affected files, updating its knowledge graph accordingly.\n\n",
	PersistentPreRunE: runInitialize,
}

func init() {
	// T024: Create logging Manager in bootstrap mode (stderr text only)
	logManager = logging.NewManager()
	slog.SetDefault(logManager.Logger())

	// Register global flags
	memorizerCmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "Suppress non-error output")

	// Register subcommands
	memorizerCmd.AddCommand(version.VersionCmd)
	memorizerCmd.AddCommand(initcmd.InitializeCmd)
	memorizerCmd.AddCommand(daemon.DaemonCmd)
	memorizerCmd.AddCommand(remember.RememberCmd)
	memorizerCmd.AddCommand(forget.ForgetCmd)
	memorizerCmd.AddCommand(list.ListCmd)
	memorizerCmd.AddCommand(read.ReadCmd)
	memorizerCmd.AddCommand(integrations.IntegrationsCmd)
	memorizerCmd.AddCommand(providers.ProvidersCmd)
	memorizerCmd.AddCommand(configcmd.ConfigCmd)
}

func runInitialize(cmd *cobra.Command, args []string) error {
	logger := logManager.Logger()

	// Initialize config subsystem
	if err := config.Init(); err != nil {
		return err
	}

	// T025: Upgrade logging after config is available
	cfg := config.Get()
	logFile := config.ExpandPath(cfg.LogFile)
	level, ok := logging.ParseLevel(cfg.LogLevel)
	if !ok {
		level = logging.DefaultLevel
		if cfg.LogLevel != "" {
			logger.Warn("invalid log level configured, using default", "configured", cfg.LogLevel, "default", "info")
		}
	}

	if err := logManager.Upgrade(logFile, level); err != nil {
		logger.Warn("failed to enable file logging, continuing with stderr only", "error", err)
		// Don't return error - continue with bootstrap mode
	}

	return nil
}

func Execute() error {
	memorizerCmd.SilenceErrors = true
	memorizerCmd.SilenceUsage = true

	// T026: Ensure logging is properly closed on exit
	defer func() { _ = logManager.Close() }()

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
			_ = cmd.Usage()
		}

		return err
	}

	return nil
}
