package read

import (
	"bytes"
	"context"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/export"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
)

func TestReadCmd_Basic(t *testing.T) {
	setupReadServer(t, readResponseOK())

	cmd := createTestCommand()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("read command failed: %v", err)
	}

	if stdout.String() != "<graph/>" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "<graph/>")
	}
	if !strings.Contains(stderr.String(), "Exported") {
		t.Errorf("expected stats in stderr, got: %s", stderr.String())
	}
}

func TestReadCmd_OutputFile(t *testing.T) {
	setupReadServer(t, readResponseOK())

	cmd := createTestCommand()
	outputPath := t.TempDir() + "/graph.xml"
	cmd.SetArgs([]string{"--output", outputPath})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("read command failed: %v", err)
	}

	if stdout.String() != "" {
		t.Errorf("expected empty stdout for file output, got: %q", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(data) != "<graph/>" {
		t.Errorf("output file = %q, want %q", string(data), "<graph/>")
	}
	if !strings.Contains(stderr.String(), "Exported") {
		t.Errorf("expected stats in stderr, got: %s", stderr.String())
	}
}

func TestReadCmd_Quiet(t *testing.T) {
	setupReadServer(t, readResponseOK())

	cmd := createTestCommand()
	cmd.SetArgs([]string{"--quiet"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("read command failed: %v", err)
	}

	if stderr.String() != "" {
		t.Errorf("expected no stderr in quiet mode, got: %q", stderr.String())
	}
}

func TestReadCmd_Error(t *testing.T) {
	setupReadServer(t, func(ctx context.Context, req daemon.ReadRequest) (*daemon.ReadResponse, error) {
		return nil, daemon.ErrReadUnavailable
	})

	cmd := createTestCommand()

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for read unavailable")
	}
	if !strings.Contains(err.Error(), "read request failed") {
		t.Errorf("error = %q, want contains %q", err.Error(), "read request failed")
	}
}

// Helper functions

type readTestServer struct {
	env *testutil.TestEnv
}

func setupReadServer(t *testing.T, fn daemon.ReadFunc) *readTestServer {
	t.Helper()

	env := testutil.NewTestEnv(t)

	server := daemon.NewServer(daemon.NewHealthManager(), daemon.ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})
	server.SetReadFunc(fn)

	httpServer := httptest.NewServer(server.Handler())
	setDaemonConfigForTest(t, httpServer.URL)

	t.Cleanup(func() {
		httpServer.Close()
	})

	return &readTestServer{env: env}
}

func readResponseOK() func(ctx context.Context, req daemon.ReadRequest) (*daemon.ReadResponse, error) {
	return func(ctx context.Context, req daemon.ReadRequest) (*daemon.ReadResponse, error) {
		return &daemon.ReadResponse{
			Output: "<graph/>",
			Stats: &export.ExportStats{
				FileCount:      1,
				DirectoryCount: 2,
				OutputSize:     8,
				Duration:       100 * time.Millisecond,
			},
		}, nil
	}
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
	readFormat = "xml"
	readEnvelope = "none"
	readOutput = ""
	readMaxFiles = 0
	readQuiet = false

	cmd := &cobra.Command{
		Use:     ReadCmd.Use,
		Short:   ReadCmd.Short,
		Long:    ReadCmd.Long,
		Example: ReadCmd.Example,
		Args:    ReadCmd.Args,
		PreRunE: ReadCmd.PreRunE,
		RunE:    ReadCmd.RunE,
	}

	cmd.Flags().StringVarP(&readFormat, "format", "f", "xml", "")
	cmd.Flags().StringVarP(&readEnvelope, "envelope", "e", "none", "")
	cmd.Flags().StringVarP(&readOutput, "output", "o", "", "")
	cmd.Flags().IntVar(&readMaxFiles, "max-files", 0, "")
	cmd.Flags().BoolVarP(&readQuiet, "quiet", "q", false, "")

	return cmd
}
