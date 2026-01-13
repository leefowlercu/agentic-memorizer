package list

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
	"github.com/spf13/cobra"
)

func TestListCmd_Empty(t *testing.T) {
	env := testutil.NewTestEnv(t)
	_ = env // ensures config is isolated

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No directories remembered") {
		t.Errorf("expected empty message, got: %s", output)
	}
}

func TestListCmd_WithPaths(t *testing.T) {
	env := testutil.NewTestEnv(t)

	// Add some paths
	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	reg.AddPath(ctx, "/projects/app1", nil)
	reg.AddPath(ctx, "/projects/app2", nil)
	reg.AddPath(ctx, "/documents", nil)
	reg.Close()

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Remembered directories (3)") {
		t.Errorf("expected 3 directories, got: %s", output)
	}
	if !strings.Contains(output, "/projects/app1") {
		t.Error("expected /projects/app1 in output")
	}
	if !strings.Contains(output, "/projects/app2") {
		t.Error("expected /projects/app2 in output")
	}
	if !strings.Contains(output, "/documents") {
		t.Error("expected /documents in output")
	}
}

func TestListCmd_Verbose(t *testing.T) {
	env := testutil.NewTestEnv(t)

	// Add path with config
	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	reg.AddPath(ctx, "/projects/myapp", &registry.PathConfig{
		SkipExtensions:  []string{".log", ".tmp"},
		SkipDirectories: []string{"node_modules"},
		SkipHidden:      true,
	})
	// Add a file state
	reg.UpdateFileState(ctx, &registry.FileState{
		Path:         "/projects/myapp/main.go",
		ContentHash:  "hash",
		MetadataHash: "meta",
		Size:         100,
		ModTime:      time.Now(),
	})
	reg.Close()

	cmd := createTestCommand()
	cmd.SetArgs([]string{"--verbose"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	// Check for verbose details
	if !strings.Contains(output, "Path:") {
		t.Error("expected Path: in verbose output")
	}
	if !strings.Contains(output, "Added:") {
		t.Error("expected Added: in verbose output")
	}
	if !strings.Contains(output, "Files Tracked:") {
		t.Error("expected Files Tracked: in verbose output")
	}
	if !strings.Contains(output, "Configuration:") {
		t.Error("expected Configuration: in verbose output")
	}
	if !strings.Contains(output, "Skip Extensions:") {
		t.Error("expected Skip Extensions: in verbose output")
	}
	if !strings.Contains(output, ".log") {
		t.Error("expected .log extension in verbose output")
	}
}

func TestListCmd_VerboseWithLastWalk(t *testing.T) {
	env := testutil.NewTestEnv(t)

	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	reg.AddPath(ctx, "/projects/myapp", nil)
	walkTime := time.Now().Add(-time.Hour)
	reg.UpdatePathLastWalk(ctx, "/projects/myapp", walkTime)
	reg.Close()

	cmd := createTestCommand()
	cmd.SetArgs([]string{"--verbose"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Last Walk:") {
		t.Error("expected Last Walk: in verbose output")
	}
	// Should not contain "never" since we set a walk time
	if strings.Contains(output, "Last Walk: never") {
		t.Error("expected actual walk time, not 'never'")
	}
}

func TestListCmd_VerboseNeverWalked(t *testing.T) {
	env := testutil.NewTestEnv(t)

	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	reg.AddPath(ctx, "/projects/myapp", nil)
	reg.Close()

	cmd := createTestCommand()
	cmd.SetArgs([]string{"--verbose"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Last Walk: never") {
		t.Error("expected 'Last Walk: never' for new path")
	}
}

func TestListCmd_VerboseWithUseVision(t *testing.T) {
	env := testutil.NewTestEnv(t)

	useVision := false
	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	reg.AddPath(ctx, "/projects/myapp", &registry.PathConfig{
		UseVision: &useVision,
	})
	reg.Close()

	cmd := createTestCommand()
	cmd.SetArgs([]string{"--verbose"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Use Vision: false") {
		t.Error("expected 'Use Vision: false' in verbose output")
	}
}

func TestFormatConfigJSON(t *testing.T) {
	cfg := &registry.PathConfig{
		SkipExtensions: []string{".log"},
		SkipHidden:     true,
	}

	result := FormatConfigJSON(cfg)
	if !strings.Contains(result, "skip_extensions") {
		t.Error("expected skip_extensions in JSON output")
	}
	if !strings.Contains(result, ".log") {
		t.Error("expected .log in JSON output")
	}
}

func TestFormatConfigJSON_Nil(t *testing.T) {
	result := FormatConfigJSON(nil)
	if result != "{}" {
		t.Errorf("expected {}, got %s", result)
	}
}

// Helper functions

func createTestCommand() *cobra.Command {
	// Reset flag variables
	listVerbose = false

	cmd := &cobra.Command{
		Use:     ListCmd.Use,
		Short:   ListCmd.Short,
		Long:    ListCmd.Long,
		Example: ListCmd.Example,
		Args:    ListCmd.Args,
		PreRunE: ListCmd.PreRunE,
		RunE:    ListCmd.RunE,
	}

	cmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "")

	return cmd
}
