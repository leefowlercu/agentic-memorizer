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
	"github.com/leefowlercu/agentic-memorizer/internal/logging"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/spf13/cobra"
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
  memorizer mcp start

  # Start with debug logging (logs to both stderr and ~/.memorizer/mcp.log)
  memorizer mcp start --log-level debug

  # View MCP logs
  tail -f ~/.memorizer/mcp.log`,
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

	// Determine log level (flag overrides config)
	logLevel := cfg.MCP.LogLevel
	if cmd.Flags().Changed("log-level") {
		logLevel, _ = cmd.Flags().GetString("log-level")
	}

	// Setup logger with dual output (stderr + file)
	logger, logWriter, err := logging.NewLogger(
		logging.WithLogFile(cfg.MCP.LogFile),
		logging.WithLogLevel(logLevel),
		logging.WithHandler(logging.HandlerText),
		logging.WithAdditionalOutputs(os.Stderr),
	)
	if err != nil {
		return fmt.Errorf("failed to setup logger; %w", err)
	}
	if logWriter != nil {
		defer logWriter.Close()
	}

	// Get daemon URL for API access (constructed from host and port)
	daemonURL := cfg.MCP.GetDaemonURL()

	// Try to fetch initial index from daemon if available
	var initialIndex *types.GraphIndex
	if daemonURL != "" {
		logger.Info("fetching initial index from daemon", "url", daemonURL)
		idx, err := fetchIndexFromDaemon(daemonURL, logger)
		if err != nil {
			logger.Warn("failed to fetch initial index from daemon; will wait for sse updates", "error", err)
		} else {
			initialIndex = idx
			logger.Info("loaded initial index from daemon", "files", len(idx.Files))
		}
	}

	// Create empty index if we couldn't fetch from daemon
	if initialIndex == nil {
		initialIndex = &types.GraphIndex{
			Files: []types.FileEntry{},
			Stats: types.IndexStats{},
		}
		logger.Info("starting with empty index; will populate from sse stream")
	}

	// Create MCP server
	server := mcp.NewServer(initialIndex, logger, daemonURL)
	if daemonURL != "" {
		logger.Info("daemon api enabled", "daemon_url", daemonURL)
	} else {
		logger.Warn("no daemon url configured; running in index-only mode without graph features")
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run server
	logger.Info("starting mcp server",
		"protocol", "stdio",
		"memory_root", cfg.MemoryRoot,
	)

	if err := server.Run(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("server error; %w", err)
	}

	// Graceful shutdown
	if err := server.Shutdown(); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("mcp server stopped")
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
