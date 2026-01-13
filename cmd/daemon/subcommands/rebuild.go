package subcommands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var (
	rebuildFull    bool
	rebuildVerbose bool
)

// RebuildCmd triggers a rebuild of the knowledge graph.
var RebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Rebuild the knowledge graph from remembered directories",
	Long: "Rebuild the knowledge graph from remembered directories.\n\n" +
		"This command triggers the daemon to re-walk all remembered directories and " +
		"update the knowledge graph. By default, it performs an incremental rebuild " +
		"that only processes files that have changed since the last analysis. Use " +
		"--full to force a complete rebuild of all files.",
	Example: `  # Incremental rebuild (only changed files)
  memorizer daemon rebuild

  # Full rebuild of all files
  memorizer daemon rebuild --full

  # Full rebuild with progress output
  memorizer daemon rebuild --full --verbose`,
	PreRunE: validateRebuild,
	RunE:    runRebuild,
}

func init() {
	RebuildCmd.Flags().BoolVar(&rebuildFull, "full", false, "Force full rebuild of all files")
	RebuildCmd.Flags().BoolVar(&rebuildVerbose, "verbose", false, "Show progress output")
}

func validateRebuild(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

// RebuildRequest is the request body for the rebuild endpoint.
type RebuildRequest struct {
	Full bool `json:"full"`
}

// RebuildResponse is the response from the rebuild endpoint.
type RebuildResponse struct {
	Status        string `json:"status"`
	FilesQueued   int    `json:"files_queued"`
	DirsProcessed int    `json:"dirs_processed"`
	Duration      string `json:"duration"`
	Error         string `json:"error,omitempty"`
}

func runRebuild(cmd *cobra.Command, args []string) error {
	// Get daemon address from config
	cfg := config.Get()
	bind := cfg.Daemon.HTTPBind
	if bind == "" || bind == "0.0.0.0" {
		bind = "127.0.0.1"
	}

	// Build request URL
	url := fmt.Sprintf("http://%s:%d/rebuild", bind, cfg.Daemon.HTTPPort)

	// Create request
	method := "POST"
	if rebuildFull {
		url += "?full=true"
	}

	client := &http.Client{
		Timeout: 5 * time.Minute, // Rebuild can take a while
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request; %w", err)
	}

	if rebuildVerbose {
		fmt.Printf("Triggering %s rebuild...\n", rebuildType())
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon; %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response; %w", err)
	}

	// Parse response
	var result RebuildResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response; %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if result.Error != "" {
			return fmt.Errorf("rebuild failed; %s", result.Error)
		}
		return fmt.Errorf("rebuild failed; status %d", resp.StatusCode)
	}

	// Output result
	if rebuildVerbose {
		fmt.Printf("Rebuild completed:\n")
		fmt.Printf("  Status: %s\n", result.Status)
		fmt.Printf("  Directories processed: %d\n", result.DirsProcessed)
		fmt.Printf("  Files queued for analysis: %d\n", result.FilesQueued)
		fmt.Printf("  Duration: %s\n", result.Duration)
	} else {
		fmt.Printf("Rebuild %s: %d files queued\n", result.Status, result.FilesQueued)
	}

	return nil
}

func rebuildType() string {
	if rebuildFull {
		return "full"
	}
	return "incremental"
}
