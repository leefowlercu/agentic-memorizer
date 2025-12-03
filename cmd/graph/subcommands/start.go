package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/docker"
	"github.com/spf13/cobra"
)

var detached bool

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the FalkorDB container",
	Long: "\nStart the FalkorDB Docker container for knowledge graph storage.\n\n" +
		"FalkorDB is required for the daemon to store file metadata, semantic analysis results, " +
		"and embeddings. The container is started using Docker and persists data in ~/.agentic-memorizer/falkordb/.\n\n" +
		"Use --detach to run the container in the background (default). Without --detach, " +
		"the container runs in foreground mode and can be stopped with Ctrl+C.",
	PreRunE: validateStart,
	RunE:    runStart,
}

func init() {
	StartCmd.Flags().BoolVarP(&detached, "detach", "d", true, "Run container in background")
}

func validateStart(cmd *cobra.Command, args []string) error {
	// Check Docker is available
	if !docker.IsAvailable() {
		return fmt.Errorf("docker not found or not running; please install Docker to use the FalkorDB knowledge graph")
	}

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

	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}

	// Check if already running
	if docker.IsFalkorDBRunning(cfg.Graph.Port) {
		fmt.Printf("FalkorDB container is already running\n")
		fmt.Printf("  Redis port: %d\n", cfg.Graph.Port)
		fmt.Printf("  Browser UI: http://localhost:3000\n")
		return nil
	}

	fmt.Printf("Starting FalkorDB container...\n")
	fmt.Printf("Data directory: %s/falkordb\n", appDir)

	opts := docker.StartOptions{
		Port:    cfg.Graph.Port,
		DataDir: fmt.Sprintf("%s/falkordb", appDir),
		Detach:  detached,
	}

	if err := docker.StartFalkorDB(opts); err != nil {
		return fmt.Errorf("failed to start FalkorDB; %w", err)
	}

	fmt.Printf("FalkorDB is ready\n")
	fmt.Printf("  Redis port: %d\n", cfg.Graph.Port)
	fmt.Printf("  Browser UI: http://localhost:3000\n")
	return nil
}
