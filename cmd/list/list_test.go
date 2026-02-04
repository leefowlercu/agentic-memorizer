package list

import (
	"bytes"
	"context"
	"net"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
)

func TestListCmd_Empty(t *testing.T) {
	setupListServer(t)

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
	srv := setupListServer(t)

	// Add some paths
	ctx := context.Background()
	srv.reg.AddPath(ctx, "/projects/app1", nil)
	srv.reg.AddPath(ctx, "/projects/app2", nil)
	srv.reg.AddPath(ctx, "/documents", nil)

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

func TestListCmd_TableHeader(t *testing.T) {
	srv := setupListServer(t)

	// Add a path so we get table output
	ctx := context.Background()
	srv.reg.AddPath(ctx, "/test/path", nil)

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()

	// Check table header includes all columns
	if !strings.Contains(output, "PATH") {
		t.Error("expected PATH column header")
	}
	if !strings.Contains(output, "STATUS") {
		t.Error("expected STATUS column header")
	}
	if !strings.Contains(output, "DISCOVERED") {
		t.Error("expected DISCOVERED column header")
	}
	if !strings.Contains(output, "SEMANTIC") {
		t.Error("expected SEMANTIC column header")
	}
	if !strings.Contains(output, "EMBEDDINGS") {
		t.Error("expected EMBEDDINGS column header")
	}
	if !strings.Contains(output, "LAST WALK") {
		t.Error("expected LAST WALK column header")
	}
}

func TestListCmd_MissingPathStatus(t *testing.T) {
	srv := setupListServer(t)

	// Add paths that don't exist on filesystem
	ctx := context.Background()
	srv.reg.AddPath(ctx, "/nonexistent/path1", nil)
	srv.reg.AddPath(ctx, "/nonexistent/path2", nil)

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()

	// Both paths should show as "missing"
	if !strings.Contains(output, "missing") {
		t.Error("expected 'missing' status for non-existent paths")
	}
}

func TestListCmd_ExistingPathStatus(t *testing.T) {
	srv := setupListServer(t)

	// Create a real temp directory
	existingDir := t.TempDir()

	ctx := context.Background()
	srv.reg.AddPath(ctx, existingDir, nil)

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()

	// The real directory should show as "ok"
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' status for existing path, got: %s", output)
	}
}

func TestListCmd_MixedPathStatus(t *testing.T) {
	srv := setupListServer(t)

	// Create one real directory and one fake path
	existingDir := t.TempDir()

	ctx := context.Background()
	srv.reg.AddPath(ctx, existingDir, nil)
	srv.reg.AddPath(ctx, "/nonexistent/path", nil)

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()

	// Should have both "ok" and "missing" statuses
	if !strings.Contains(output, "ok") {
		t.Error("expected 'ok' status for existing path")
	}
	if !strings.Contains(output, "missing") {
		t.Error("expected 'missing' status for non-existent path")
	}
}

func TestListCmd_InaccessiblePathShowsDash(t *testing.T) {
	srv := setupListServer(t)

	// Add a path that doesn't exist
	ctx := context.Background()
	srv.reg.AddPath(ctx, "/nonexistent/inaccessible", nil)

	cmd := createTestCommand()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()
	lines := strings.Split(output, "\n")

	// Find the line with our path
	var pathLine string
	for _, line := range lines {
		if strings.Contains(line, "inaccessible") {
			pathLine = line
			break
		}
	}

	if pathLine == "" {
		t.Fatal("expected to find path line in output")
	}

	// For inaccessible paths, counts should show "-"
	// The output format is: PATH (40) STATUS (10) DISCOVERED (10) SEMANTIC (10) EMBEDDINGS (10) LAST WALK
	// Split by whitespace to verify the dash is present
	if !strings.Contains(pathLine, "-") {
		t.Error("expected '-' for counts/LAST WALK of inaccessible path")
	}
}

func TestListCmd_Verbose(t *testing.T) {
	srv := setupListServer(t)

	// Add path with config
	ctx := context.Background()
	srv.reg.AddPath(ctx, "/projects/myapp", &registry.PathConfig{
		SkipExtensions:  []string{".log", ".tmp"},
		SkipDirectories: []string{"node_modules"},
		SkipHidden:      true,
	})
	// Add a file state
	srv.reg.UpdateFileState(ctx, &registry.FileState{
		Path:         "/projects/myapp/main.go",
		ContentHash:  "hash",
		MetadataHash: "meta",
		Size:         100,
		ModTime:      time.Now(),
	})

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
	if !strings.Contains(output, "Files Discovered:") {
		t.Error("expected Files Discovered: in verbose output")
	}
	if !strings.Contains(output, "Files Semantic Analyzed:") {
		t.Error("expected Files Semantic Analyzed: in verbose output")
	}
	if !strings.Contains(output, "Files Embeddings Generated:") {
		t.Error("expected Files Embeddings Generated: in verbose output")
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

func TestListCmd_VerboseWithStatus(t *testing.T) {
	srv := setupListServer(t)

	// Create a real directory and a fake path
	existingDir := t.TempDir()

	ctx := context.Background()
	srv.reg.AddPath(ctx, existingDir, nil)
	srv.reg.AddPath(ctx, "/nonexistent/path", nil)

	cmd := createTestCommand()
	cmd.SetArgs([]string{"--verbose"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := stdout.String()

	// Verbose output should include Status: field
	if !strings.Contains(output, "Status:") {
		t.Error("expected 'Status:' in verbose output")
	}
	// Should have both status values
	if !strings.Contains(output, "Status: ok") {
		t.Error("expected 'Status: ok' in verbose output for existing path")
	}
	if !strings.Contains(output, "Status: missing") {
		t.Error("expected 'Status: missing' in verbose output for non-existent path")
	}
}

func TestListCmd_VerboseWithLastWalk(t *testing.T) {
	srv := setupListServer(t)

	ctx := context.Background()
	srv.reg.AddPath(ctx, "/projects/myapp", nil)
	walkTime := time.Now().Add(-time.Hour)
	srv.reg.UpdatePathLastWalk(ctx, "/projects/myapp", walkTime)

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
	srv := setupListServer(t)

	ctx := context.Background()
	srv.reg.AddPath(ctx, "/projects/myapp", nil)

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
	srv := setupListServer(t)

	useVision := false
	ctx := context.Background()
	srv.reg.AddPath(ctx, "/projects/myapp", &registry.PathConfig{
		UseVision: &useVision,
	})

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

type listTestServer struct {
	env *testutil.TestEnv
	reg *registry.SQLiteRegistry
}

func setupListServer(t *testing.T) *listTestServer {
	t.Helper()

	env := testutil.NewTestEnv(t)

	ctx := context.Background()
	reg, err := registry.Open(ctx, env.RegistryPath())
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}

	service := daemon.NewListService(reg)
	server := daemon.NewServer(daemon.NewHealthManager(), daemon.ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})
	server.SetListFunc(service.List)

	httpServer := httptest.NewServer(server.Handler())
	setDaemonConfigForTest(t, httpServer.URL)

	t.Cleanup(func() {
		httpServer.Close()
		_ = reg.Close()
	})

	return &listTestServer{env: env, reg: reg}
}

func setDaemonConfigForTest(t *testing.T, baseURL string) {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}

	host, portStr, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("failed to parse server host: %v", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse server port: %v", err)
	}

	cfg := config.Get()
	cfg.Daemon.HTTPBind = host
	cfg.Daemon.HTTPPort = port
}

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
