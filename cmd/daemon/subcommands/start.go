package subcommands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/spf13/cobra"
)

// StartCmd starts the daemon in foreground mode.
var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in foreground mode",
	Long: "Start the daemon in foreground mode.\n\n" +
		"The daemon will run in the foreground, writing logs to the configured log file " +
		"and exposing health check endpoints. Use standard backgrounding methods like " +
		"'&', 'nohup', or platform-specific service runners (launchd, systemd) to run " +
		"the daemon in the background.",
	Example: `  # Start daemon in foreground
  memorizer daemon start

  # Start daemon in background
  memorizer daemon start &

  # Start daemon with nohup
  nohup memorizer daemon start &`,
	PreRunE: validateStart,
	RunE:    runStart,
}

func validateStart(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	// Build daemon configuration from typed config
	appCfg := config.Get()
	cfg := daemon.DaemonConfig{
		HTTPPort:        appCfg.Daemon.HTTPPort,
		HTTPBind:        appCfg.Daemon.HTTPBind,
		ShutdownTimeout: time.Duration(appCfg.Daemon.ShutdownTimeout) * time.Second,
		PIDFile:         config.ExpandPath(appCfg.Daemon.PIDFile),
	}

	// Create daemon
	d := daemon.NewDaemon(cfg)

	// Create context that cancels on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize orchestrator and wire components
	orchestrator := daemon.NewOrchestrator(d)
	if err := orchestrator.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize components; %w", err)
	}

	// Start orchestrated components
	if err := orchestrator.Start(ctx); err != nil {
		return fmt.Errorf("failed to start components; %w", err)
	}

	// Update daemon health with component statuses
	d.UpdateComponentHealth(orchestrator.ComponentStatuses())

	// Ensure orchestrator cleanup on exit
	defer orchestrator.Stop(ctx)

	// Start daemon (blocking)
	slog.Info("starting daemon",
		"http_bind", cfg.HTTPBind,
		"http_port", cfg.HTTPPort,
		"pid_file", cfg.PIDFile,
	)

	if err := d.Start(ctx); err != nil {
		return fmt.Errorf("daemon error; %w", err)
	}

	return nil
}
