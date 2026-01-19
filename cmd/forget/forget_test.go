package forget

import (
	"bytes"
	"context"
	"net"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
)

func TestForgetCmd_Basic(t *testing.T) {
	server := setupForgetServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// First, add a path to the registry
	ctx := context.Background()
	reg, err := registry.Open(ctx, server.env.RegistryPath())
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	reg.AddPath(ctx, testDir, nil)
	reg.Close()

	// Execute forget command
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("forget command failed: %v", err)
	}

	// Verify path was removed from registry
	reg, _ = registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	_, err = reg.GetPath(ctx, testDir)
	if err != registry.ErrPathNotFound {
		t.Error("expected path to be removed from registry")
	}
}

func TestForgetCmd_WithFileStates(t *testing.T) {
	server := setupForgetServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Add path and file states
	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	reg.AddPath(ctx, testDir, nil)
	reg.UpdateFileState(ctx, &registry.FileState{
		Path:         testDir + "/file1.go",
		ContentHash:  "hash1",
		MetadataHash: "meta1",
		Size:         100,
		ModTime:      time.Now(),
	})
	reg.UpdateFileState(ctx, &registry.FileState{
		Path:         testDir + "/file2.go",
		ContentHash:  "hash2",
		MetadataHash: "meta2",
		Size:         200,
		ModTime:      time.Now(),
	})
	reg.Close()

	// Forget the directory
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir})
	cmd.Execute()

	// Verify file states were deleted
	reg, _ = registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	states, _ := reg.ListFileStates(ctx, testDir)
	if len(states) != 0 {
		t.Errorf("expected 0 file states, got %d", len(states))
	}
}

func TestForgetCmd_KeepData(t *testing.T) {
	server := setupForgetServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Add path and file states
	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	reg.AddPath(ctx, testDir, nil)
	reg.UpdateFileState(ctx, &registry.FileState{
		Path:         testDir + "/file1.go",
		ContentHash:  "hash1",
		MetadataHash: "meta1",
		Size:         100,
		ModTime:      time.Now(),
	})
	reg.Close()

	// Forget with --keep-data
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--keep-data"})
	cmd.Execute()

	// Verify path was removed but file states preserved
	reg, _ = registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	_, err := reg.GetPath(ctx, testDir)
	if err != registry.ErrPathNotFound {
		t.Error("expected path to be removed")
	}

	states, _ := reg.ListFileStates(ctx, testDir)
	if len(states) != 1 {
		t.Errorf("expected 1 file state (preserved), got %d", len(states))
	}
}

func TestForgetCmd_NotRemembered(t *testing.T) {
	server := setupForgetServer(t)
	testDir := server.env.CreateTestDir("testproject")

	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for path that isn't remembered")
	}
}

// Helper functions

type forgetTestServer struct {
	env *testutil.TestEnv
}

func setupForgetServer(t *testing.T) *forgetTestServer {
	t.Helper()

	env := testutil.NewTestEnv(t)

	ctx := context.Background()
	reg, err := registry.Open(ctx, env.RegistryPath())
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}

	bus := events.NewBus()
	service := daemon.NewRememberService(reg, bus, config.Get().Defaults)

	server := daemon.NewServer(daemon.NewHealthManager(), daemon.ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})
	server.SetRememberFunc(service.Remember)
	server.SetForgetFunc(service.Forget)

	httpServer := httptest.NewServer(server.Handler())
	setDaemonConfigForTest(t, httpServer.URL)

	t.Cleanup(func() {
		httpServer.Close()
		bus.Close()
		reg.Close()
	})

	return &forgetTestServer{env: env}
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
	forgetKeepData = false

	cmd := &cobra.Command{
		Use:     ForgetCmd.Use,
		Short:   ForgetCmd.Short,
		Long:    ForgetCmd.Long,
		Example: ForgetCmd.Example,
		Args:    ForgetCmd.Args,
		PreRunE: ForgetCmd.PreRunE,
		RunE:    ForgetCmd.RunE,
	}

	cmd.Flags().BoolVar(&forgetKeepData, "keep-data", false, "")

	return cmd
}
