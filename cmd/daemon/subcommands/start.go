package subcommands

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in foreground",
	Long: "\nStart the background indexing daemon in foreground mode.\n\n" +
		"The daemon will continuously monitor the memory directory and rebuild " +
		"the index as needed. Press Ctrl+C to stop the daemon.\n\n" +
		"A PID file is created at ~/.agentic-memorizer/daemon.pid to track the running " +
		"daemon. If you encounter 'daemon already running' errors, check the PID file.",
	PreRunE: validateStart,
	RunE:    runStart,
}

func validateStart(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	if !cfg.Daemon.Enabled {
		return fmt.Errorf("daemon is disabled in configuration (set daemon.enabled: true)")
	}

	// Setup logger
	logger, logWriter, err := setupLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup logger; %w", err)
	}

	// Create daemon instance
	d, err := daemon.New(cfg, logger, logWriter)
	if err != nil {
		return fmt.Errorf("failed to create daemon; %w", err)
	}

	// Start daemon (blocks until stopped)
	fmt.Println("Starting daemon in foreground mode...")
	fmt.Println("Press Ctrl+C to stop")
	return d.Start()
}

// setupLogger creates a logger based on configuration
func setupLogger(cfg *config.Config) (*slog.Logger, *lumberjack.Logger, error) {
	var level slog.Level
	switch cfg.Daemon.LogLevel {
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

	// Create log file if specified
	var handler slog.Handler
	var logWriter *lumberjack.Logger
	if cfg.Daemon.LogFile != "" {
		logDir := cfg.Daemon.LogFile[:len(cfg.Daemon.LogFile)-len("/daemon.log")]
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory; %w", err)
		}

		// Use lumberjack for log rotation
		logWriter = &lumberjack.Logger{
			Filename:   cfg.Daemon.LogFile,
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			MaxAge:     28, // days
			Compress:   true,
		}

		handler = slog.NewJSONHandler(logWriter, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	return slog.New(handler), logWriter, nil
}
