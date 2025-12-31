package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/docker"
	"github.com/spf13/cobra"
)

var removeContainer bool

var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the FalkorDB Docker container gracefully",
	Long: "\nStop the FalkorDB Docker container.\n\n" +
		"This gracefully stops the FalkorDB container. Data is preserved in the persistent volume " +
		"and will be available when the container is started again.\n\n" +
		"Use --remove to also remove the container (data is still preserved in the volume).",
	Example: `  # Stop the FalkorDB container
  memorizer graph stop

  # Stop and remove the container
  memorizer graph stop --remove`,
	PreRunE: validateStop,
	RunE:    runStop,
}

func init() {
	StopCmd.Flags().BoolVarP(&removeContainer, "remove", "r", false, "Remove container after stopping")
}

func validateStop(cmd *cobra.Command, args []string) error {
	if !docker.IsAvailable() {
		return fmt.Errorf("docker not found or not running; please install Docker")
	}

	cmd.SilenceUsage = true
	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	// Check if container exists
	if !docker.ContainerExists() {
		fmt.Printf("FalkorDB container does not exist\n")
		return nil
	}

	// Check if container is running
	if docker.IsFalkorDBRunning(0) {
		fmt.Printf("Stopping FalkorDB container...\n")
		if err := docker.StopFalkorDB(); err != nil {
			return err
		}
		fmt.Printf("FalkorDB container stopped\n")
	} else {
		fmt.Printf("FalkorDB container is not running\n")
	}

	if removeContainer {
		fmt.Printf("Removing FalkorDB container...\n")
		if err := docker.RemoveFalkorDB(); err != nil {
			return err
		}
		fmt.Printf("FalkorDB container removed (data preserved in volume)\n")
	}

	return nil
}
