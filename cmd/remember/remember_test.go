package remember

import (
	"bytes"
	"context"
	"net"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
)

func TestRememberCmd_Basic(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	// Verify path was added to registry
	ctx := context.Background()
	reg, err := registry.Open(ctx, server.env.RegistryPath())
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	rp, err := reg.GetPath(ctx, testDir)
	if err != nil {
		t.Fatalf("path not found in registry: %v", err)
	}
	if rp.Path != testDir {
		t.Errorf("expected path %q, got %q", testDir, rp.Path)
	}
}

func TestRememberCmd_WithSkipFlags(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Use --set-skip-ext to replace defaults entirely
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--set-skip-ext=.log,.tmp", "--skip-hidden=false"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	// Verify config was stored
	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)
	if rp.Config == nil {
		t.Fatal("expected config to be set")
	}
	if len(rp.Config.SkipExtensions) != 2 {
		t.Errorf("expected 2 skip extensions, got %d", len(rp.Config.SkipExtensions))
	}
	if rp.Config.SkipHidden {
		t.Error("expected SkipHidden to be false")
	}
}

func TestRememberCmd_WithIncludeFlags(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--add-include-file=.env,.envrc"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)
	if len(rp.Config.IncludeFiles) != 2 {
		t.Errorf("expected 2 include files, got %d", len(rp.Config.IncludeFiles))
	}
}

func TestRememberCmd_NonExistentPath(t *testing.T) {
	server := setupRememberServer(t)
	nonExistent := server.env.ConfigDir + "/doesnotexist"

	cmd := createTestCommand()
	cmd.SetArgs([]string{nonExistent})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestRememberCmd_FileInsteadOfDir(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")
	testFile := server.env.CreateTestFile(testDir, "file.txt", "content")

	cmd := createTestCommand()
	cmd.SetArgs([]string{testFile})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for file path")
	}
}

func TestRememberCmd_DuplicatePath(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Remember once
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir})
	cmd1.Execute()

	// Remember again
	cmd2 := createTestCommand()
	cmd2.SetArgs([]string{testDir})
	err := cmd2.Execute()

	if err == nil {
		t.Fatal("expected error for duplicate path")
	}
}

func TestRememberCmd_UseVisionFlag(t *testing.T) {
	tests := []struct {
		name      string
		flag      string
		wantValue *bool
		wantErr   bool
	}{
		{
			name:      "use-vision true",
			flag:      "--use-vision=true",
			wantValue: boolPtr(true),
		},
		{
			name:      "use-vision false",
			flag:      "--use-vision=false",
			wantValue: boolPtr(false),
		},
		{
			name:    "use-vision invalid",
			flag:    "--use-vision=maybe",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupRememberServer(t)
			testDir := server.env.CreateTestDir("testproject")

			cmd := createTestCommand()
			cmd.SetArgs([]string{testDir, tt.flag})

			err := cmd.Execute()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			ctx := context.Background()
			reg, _ := registry.Open(ctx, server.env.RegistryPath())
			defer reg.Close()

			rp, _ := reg.GetPath(ctx, testDir)
			if tt.wantValue == nil && rp.Config.UseVision != nil {
				t.Error("expected UseVision to be nil")
			}
			if tt.wantValue != nil {
				if rp.Config.UseVision == nil {
					t.Fatal("expected UseVision to be set")
				}
				if *rp.Config.UseVision != *tt.wantValue {
					t.Errorf("expected UseVision %v, got %v", *tt.wantValue, *rp.Config.UseVision)
				}
			}
		})
	}
}

func TestRememberCmd_DefaultsApplied(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Remember with no flags - defaults should be applied
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)
	if rp.Config == nil {
		t.Fatal("expected config to be set")
	}

	// Defaults should be applied - check that skip extensions includes defaults
	if len(rp.Config.SkipExtensions) == 0 {
		t.Error("expected default skip extensions to be applied")
	}
	if len(rp.Config.SkipDirectories) == 0 {
		t.Error("expected default skip directories to be applied")
	}
	if len(rp.Config.SkipFiles) == 0 {
		t.Error("expected default skip files to be applied")
	}
	if !rp.Config.SkipHidden {
		t.Error("expected SkipHidden to be true (default)")
	}
}

func TestRememberCmd_AddSkipExtMergesDefaults(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Add a unique extension not in defaults
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--add-skip-ext=.myext"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)

	// Should have defaults + .myext
	hasMyext := false
	hasExe := false
	for _, ext := range rp.Config.SkipExtensions {
		if ext == ".myext" {
			hasMyext = true
		}
		if ext == ".exe" {
			hasExe = true
		}
	}
	if !hasMyext {
		t.Error("expected .myext to be in skip extensions")
	}
	if !hasExe {
		t.Error("expected default .exe to still be in skip extensions")
	}
}

func TestRememberCmd_SetSkipExtReplacesDefaults(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Set extensions - should replace defaults
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--set-skip-ext=.only"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)

	// Should have only .only, no defaults
	if len(rp.Config.SkipExtensions) != 1 {
		t.Errorf("expected 1 skip extension, got %d", len(rp.Config.SkipExtensions))
	}
	if rp.Config.SkipExtensions[0] != ".only" {
		t.Errorf("expected .only, got %s", rp.Config.SkipExtensions[0])
	}
}

// Helper functions

type rememberTestServer struct {
	env *testutil.TestEnv
}

func setupRememberServer(t *testing.T) *rememberTestServer {
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

	return &rememberTestServer{env: env}
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
	rememberAddSkipExt = nil
	rememberSetSkipExt = nil
	rememberAddSkipDir = nil
	rememberSetSkipDir = nil
	rememberAddSkipFiles = nil
	rememberSetSkipFiles = nil
	rememberAddIncludeExt = nil
	rememberAddIncludeDir = nil
	rememberAddIncludeFile = nil
	rememberSkipHidden = true
	rememberUseVision = nil
	useVisionFlag = ""

	// Create a fresh command
	cmd := &cobra.Command{
		Use:     RememberCmd.Use,
		Short:   RememberCmd.Short,
		Long:    RememberCmd.Long,
		Example: RememberCmd.Example,
		Args:    RememberCmd.Args,
		PreRunE: RememberCmd.PreRunE,
		RunE:    RememberCmd.RunE,
	}

	// Re-add flags
	cmd.Flags().StringSliceVar(&rememberAddSkipExt, "add-skip-ext", nil, "")
	cmd.Flags().StringSliceVar(&rememberAddSkipDir, "add-skip-dir", nil, "")
	cmd.Flags().StringSliceVar(&rememberAddSkipFiles, "add-skip-file", nil, "")
	cmd.Flags().StringSliceVar(&rememberSetSkipExt, "set-skip-ext", nil, "")
	cmd.Flags().StringSliceVar(&rememberSetSkipDir, "set-skip-dir", nil, "")
	cmd.Flags().StringSliceVar(&rememberSetSkipFiles, "set-skip-file", nil, "")
	cmd.Flags().StringSliceVar(&rememberAddIncludeExt, "add-include-ext", nil, "")
	cmd.Flags().StringSliceVar(&rememberAddIncludeDir, "add-include-dir", nil, "")
	cmd.Flags().StringSliceVar(&rememberAddIncludeFile, "add-include-file", nil, "")
	cmd.Flags().BoolVar(&rememberSkipHidden, "skip-hidden", true, "")
	cmd.Flags().StringVar(&useVisionFlag, "use-vision", "", "")

	return cmd
}

func boolPtr(b bool) *bool {
	return &b
}

func TestRememberCmd_UpdateExistingPath(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Remember once with initial config
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir, "--set-skip-ext=.exe"})
	err := cmd1.Execute()
	if err != nil {
		t.Fatalf("first remember failed: %v", err)
	}

	// Verify initial config
	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	rp1, _ := reg.GetPath(ctx, testDir)
	reg.Close()

	if len(rp1.Config.SkipExtensions) != 1 || rp1.Config.SkipExtensions[0] != ".exe" {
		t.Fatalf("initial config incorrect: %v", rp1.Config.SkipExtensions)
	}

	// Remember again with modification flags - should update, not error
	cmd2 := createTestCommand()
	cmd2.SetArgs([]string{testDir, "--add-skip-ext=.dll"})

	err = cmd2.Execute()
	if err != nil {
		t.Fatalf("update remember failed: %v", err)
	}

	// Verify config was updated
	reg2, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg2.Close()

	rp2, _ := reg2.GetPath(ctx, testDir)

	// Should have both .exe and .dll
	if len(rp2.Config.SkipExtensions) != 2 {
		t.Errorf("expected 2 skip extensions, got %d: %v", len(rp2.Config.SkipExtensions), rp2.Config.SkipExtensions)
	}

	hasExe := false
	hasDll := false
	for _, ext := range rp2.Config.SkipExtensions {
		if ext == ".exe" {
			hasExe = true
		}
		if ext == ".dll" {
			hasDll = true
		}
	}
	if !hasExe {
		t.Error("expected .exe to be preserved")
	}
	if !hasDll {
		t.Error("expected .dll to be added")
	}
}

func TestRememberCmd_MultipleAddFlags(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Remember with initial config
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir, "--set-skip-ext=.exe"})
	cmd1.Execute()

	// Update with multiple --add-* flags
	cmd2 := createTestCommand()
	cmd2.SetArgs([]string{
		testDir,
		"--add-skip-ext=.dll",
		"--add-skip-dir=vendor",
		"--add-include-file=.env",
	})

	err := cmd2.Execute()
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)

	// Should have .exe and .dll
	if len(rp.Config.SkipExtensions) != 2 {
		t.Errorf("expected 2 skip extensions, got %d: %v", len(rp.Config.SkipExtensions), rp.Config.SkipExtensions)
	}

	// Should have vendor in skip dirs
	hasVendor := false
	for _, dir := range rp.Config.SkipDirectories {
		if dir == "vendor" {
			hasVendor = true
		}
	}
	if !hasVendor {
		t.Error("expected vendor in skip directories")
	}

	// Should have .env in include files
	hasEnv := false
	for _, f := range rp.Config.IncludeFiles {
		if f == ".env" {
			hasEnv = true
		}
	}
	if !hasEnv {
		t.Error("expected .env in include files")
	}
}

func TestRememberCmd_AddSkipDir(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Remember with initial config that has some skip dirs
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir, "--set-skip-dir=node_modules"})
	cmd1.Execute()

	// Update with --add-skip-dir
	cmd2 := createTestCommand()
	cmd2.SetArgs([]string{testDir, "--add-skip-dir=vendor,dist"})

	err := cmd2.Execute()
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)

	// Should have node_modules, vendor, and dist
	if len(rp.Config.SkipDirectories) != 3 {
		t.Errorf("expected 3 skip directories, got %d: %v", len(rp.Config.SkipDirectories), rp.Config.SkipDirectories)
	}

	expected := map[string]bool{"node_modules": true, "vendor": true, "dist": true}
	for _, dir := range rp.Config.SkipDirectories {
		if !expected[dir] {
			t.Errorf("unexpected directory in list: %s", dir)
		}
	}
}

func TestRememberCmd_SetSkipDirReplacesExisting(t *testing.T) {
	server := setupRememberServer(t)
	testDir := server.env.CreateTestDir("testproject")

	// Remember with initial config
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir, "--set-skip-dir=node_modules,vendor"})
	cmd1.Execute()

	// Update with --set-skip-dir (replaces all)
	cmd2 := createTestCommand()
	cmd2.SetArgs([]string{testDir, "--set-skip-dir=dist"})

	err := cmd2.Execute()
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, server.env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)

	// Should have only dist
	if len(rp.Config.SkipDirectories) != 1 {
		t.Errorf("expected 1 skip directory, got %d: %v", len(rp.Config.SkipDirectories), rp.Config.SkipDirectories)
	}
	if rp.Config.SkipDirectories[0] != "dist" {
		t.Errorf("expected dist, got %s", rp.Config.SkipDirectories[0])
	}
}
