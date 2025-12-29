package subcommands

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/cmd/shared"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/logging"
	"github.com/spf13/cobra"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in foreground",
	Long: "\nStart the background indexing daemon in foreground mode.\n\n" +
		"The daemon will continuously monitor the memory directory and rebuild " +
		"the index as needed. Press Ctrl+C to stop the daemon.\n\n" +
		"A PID file is created at ~/.memorizer/daemon.pid to track the running " +
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
	cfg, err := shared.GetConfig()
	if err != nil {
		return err
	}

	// Setup logger with centralized factory (using Text handler for human-readable logs)
	logger, logWriter, err := logging.NewLogger(
		logging.WithLogFile(cfg.Daemon.LogFile),
		logging.WithLogLevel(cfg.Daemon.LogLevel),
		logging.WithHandler(logging.HandlerText),
	)
	if err != nil {
		return fmt.Errorf("failed to setup logger; %w", err)
	}

	// Create daemon instance
	d, err := daemon.New(cfg, logger, logWriter)
	if err != nil {
		return fmt.Errorf("failed to create daemon; %w", err)
	}

	// Start daemon (blocks until stopped)
	sm := daemon.DetectServiceManager()
	managed := daemon.IsServiceManaged()

	if managed {
		fmt.Println("Starting daemon (service-managed)...")
	} else {
		fmt.Println("Starting daemon in foreground mode...")
		fmt.Println("Press Ctrl+C to stop")
		fmt.Print(daemon.GetServiceManagerHint(sm))
	}

	// Start daemon and wrap "already running" errors with helpful context
	if err := d.Start(); err != nil {
		// Check if this is an "already running" error
		if strings.Contains(err.Error(), "daemon already running") {
			return fmt.Errorf("%w\n\n%s", err, daemon.GetAlreadyRunningHelp())
		}
		return err
	}

	return nil
}
