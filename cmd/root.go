package cmd

import (
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/logging"
	"github.com/spf13/cobra"
)

// logManager is the global logging manager, created in init() and upgraded after config loads
var logManager *logging.Manager

var memorizerCmd = &cobra.Command{
	Use:   "memorizer",
	Short: "A Knowledge Graph Based Memorization Tool for AI Agents",
	Long: "Agentic Memorizer provides automatic awareness and analysis capabilities for files contained in registered directories.\n\n" +
		"A background daemon monitors registered directories and, optionally, their subdirectories for file additions, updates, moves, and deletions. " +
		"When changes are detected, the tool performs metadata extraction, semantic analysis, and embeddings generation for the affected files, updating its knowledge graph accordingly.\n\n",
	PersistentPreRunE: runInitialize,
}

func init() {
	// T024: Create logging Manager in bootstrap mode (stderr text only)
	logManager = logging.NewManager()
}

func runInitialize(cmd *cobra.Command, args []string) error {
	logger := logManager.Logger()

	// Initialize config subsystem
	if err := config.Init(); err != nil {
		return err
	}

	// T025: Upgrade logging after config is available
	logFile := config.GetPath("log_file")
	levelStr := config.GetString("log_level")
	level, ok := logging.ParseLevel(levelStr)
	if !ok {
		level = logging.DefaultLevel
		if levelStr != "" {
			logger.Warn("invalid log level configured, using default", "configured", levelStr, "default", "info")
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
