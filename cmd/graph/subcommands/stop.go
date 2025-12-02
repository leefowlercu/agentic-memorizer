package subcommands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var removeContainer bool

var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the FalkorDB container",
	Long: "\nStop the FalkorDB Docker container.\n\n" +
		"This gracefully stops the FalkorDB container. Data is preserved in the persistent volume " +
		"and will be available when the container is started again.\n\n" +
		"Use --remove to also remove the container (data is still preserved in the volume).",
	PreRunE: validateStop,
	RunE:    runStop,
}

func init() {
	StopCmd.Flags().BoolVarP(&removeContainer, "remove", "r", false, "Remove container after stopping")
}

func validateStop(cmd *cobra.Command, args []string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found; please install Docker")
	}

	cmd.SilenceUsage = true
	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	containerName := "memorizer-falkordb"

	// Check if container exists
	checkCmd := exec.Command("docker", "inspect", containerName)
	if err := checkCmd.Run(); err != nil {
		fmt.Printf("FalkorDB container is not running\n")
		return nil
	}

	// Check if container is running
	statusCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check container status; %w", err)
	}

	if strings.TrimSpace(string(output)) == "true" {
		fmt.Printf("Stopping FalkorDB container...\n")
		stopCmd := exec.Command("docker", "stop", containerName)
		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop container; %w", err)
		}
		fmt.Printf("FalkorDB container stopped\n")
	} else {
		fmt.Printf("FalkorDB container is not running\n")
	}

	if removeContainer {
		fmt.Printf("Removing FalkorDB container...\n")
		rmCmd := exec.Command("docker", "rm", containerName)
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove container; %w", err)
		}
		fmt.Printf("FalkorDB container removed (data preserved in volume)\n")
	}

	return nil
}
