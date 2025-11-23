package subcommands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MCP server",
	Long: "\nStart the MCP (Model Context Protocol) server for integration with AI tools\n" +
		"like GitHub Copilot CLI and Claude Code.\n\n" +
		"The server communicates via stdio using JSON-RPC 2.0 protocol and provides:\n" +
		"- Resources: Semantic file index in multiple formats\n" +
		"- Tools: Search, metadata lookup, and recent files queries (Phase 3)\n" +
		"- Prompts: Templated workflows (Phase 5)\n\n" +
		"The server reads from the precomputed index maintained by the background daemon. " +
		"Ensure the daemon is running for up-to-date results.",
	Example: `  # Start MCP server (typically invoked by AI tool, not manually)
  agentic-memorizer mcp start

  # Start with debug logging (logs to both stderr and ~/.agentic-memorizer/mcp.log)
  agentic-memorizer mcp start --log-level debug

  # View MCP logs
  tail -f ~/.agentic-memorizer/mcp.log`,
	PreRunE: validateStart,
	RunE:    runStart,
}

func init() {
	StartCmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
}

func validateStart(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	// Initialize config
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	// Setup logger with dual output (stderr + file)
	logger, logWriter, err := setupLogger(cmd, cfg)
	if err != nil {
		return fmt.Errorf("failed to setup logger; %w", err)
	}
	if logWriter != nil {
		defer logWriter.Close()
	}

	// Load precomputed index
	indexPath, err := config.GetIndexPath()
	if err != nil {
		return fmt.Errorf("failed to get index path; %w", err)
	}

	indexManager := index.NewManager(indexPath)
	computed, err := indexManager.LoadComputed()
	if err != nil {
		return fmt.Errorf("failed to load precomputed index; %w\n\nEnsure the daemon is running: %s daemon start", err, os.Args[0])
	}

	logger.Info("Loaded precomputed index",
		"files", computed.Index.Stats.TotalFiles,
		"analyzed", computed.Index.Stats.AnalyzedFiles,
		"cached", computed.Index.Stats.CachedFiles,
	)

	// Create MCP server with optional SSE client
	server := mcp.NewServer(computed.Index, logger, cfg.MCP.DaemonSSEURL)
	if cfg.MCP.DaemonSSEURL != "" {
		logger.Info("SSE client enabled", "daemon_url", cfg.MCP.DaemonSSEURL)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run server
	logger.Info("Starting MCP server",
		"protocol", "stdio",
		"memory_root", cfg.MemoryRoot,
	)

	if err := server.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("server error; %w", err)
	}

	// Graceful shutdown
	if err := server.Shutdown(); err != nil {
		logger.Error("Shutdown error", "error", err)
	}

	logger.Info("MCP server stopped")
	return nil
}

// setupLogger creates a logger that writes to both stderr and file
func setupLogger(cmd *cobra.Command, cfg *config.Config) (*slog.Logger, *lumberjack.Logger, error) {
	// Determine log level (flag overrides config)
	logLevel := cfg.MCP.LogLevel
	if cmd.Flags().Changed("log-level") {
		logLevel, _ = cmd.Flags().GetString("log-level")
	}

	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create log file with lumberjack rotation
	if cfg.MCP.LogFile != "" {
		logDir := cfg.MCP.LogFile[:len(cfg.MCP.LogFile)-len("/mcp.log")]
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory; %w", err)
		}

		logWriter := &lumberjack.Logger{
			Filename:   cfg.MCP.LogFile,
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			MaxAge:     28, // days
			Compress:   true,
		}

		// Use multi-writer to write to both stderr and file
		// Stderr: text format for client logs
		// File: JSON format for structured debugging
		multiWriter := io.MultiWriter(os.Stderr, logWriter)

		handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
			Level: level,
		})

		return slog.New(handler), logWriter, nil
	}

	// Fallback to stderr only if no log file configured
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler), nil, nil
}
