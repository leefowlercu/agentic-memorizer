package subcommands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
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
		"- Tools: Search, metadata lookup, and recent files queries\n" +
		"- Prompts: Templated workflows\n\n" +
		"The server gets data from the daemon HTTP API. Ensure the daemon is running " +
		"with HTTP port enabled for full functionality.",
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

	// Determine daemon URL (prefer daemon_url, fall back to daemon_sse_url for backward compatibility)
	daemonURL := cfg.MCP.DaemonURL
	if daemonURL == "" && cfg.MCP.DaemonSSEURL != "" {
		// Try to derive base URL from SSE URL
		// e.g., http://localhost:8080/notifications/stream -> http://localhost:8080
		daemonURL = cfg.MCP.DaemonSSEURL
		for _, suffix := range []string{"/notifications/stream", "/sse", "/"} {
			if len(daemonURL) > len(suffix) && daemonURL[len(daemonURL)-len(suffix):] == suffix {
				daemonURL = daemonURL[:len(daemonURL)-len(suffix)]
				break
			}
		}
	}

	// Try to fetch initial index from daemon if available
	var initialIndex *types.GraphIndex
	if daemonURL != "" {
		logger.Info("Fetching initial index from daemon", "url", daemonURL)
		idx, err := fetchIndexFromDaemon(daemonURL, logger)
		if err != nil {
			logger.Warn("Failed to fetch initial index from daemon; will wait for SSE updates", "error", err)
		} else {
			initialIndex = idx
			logger.Info("Loaded initial index from daemon", "files", len(idx.Files))
		}
	}

	// Create empty index if we couldn't fetch from daemon
	if initialIndex == nil {
		initialIndex = &types.GraphIndex{
			Files: []types.FileEntry{},
			Stats: types.IndexStats{},
		}
		logger.Info("Starting with empty index; will populate from SSE stream")
	}

	// Create MCP server
	server := mcp.NewServer(initialIndex, logger, daemonURL)
	if daemonURL != "" {
		logger.Info("Daemon API enabled", "daemon_url", daemonURL)
	} else {
		logger.Warn("No daemon URL configured; running in index-only mode without graph features")
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

// fetchIndexFromDaemon fetches the current index from the daemon API
func fetchIndexFromDaemon(daemonURL string, logger *slog.Logger) (*types.GraphIndex, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(daemonURL + "/api/v1/index")
	if err != nil {
		return nil, fmt.Errorf("request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response; %w", err)
	}

	var idx types.GraphIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse index; %w", err)
	}

	return &idx, nil
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
