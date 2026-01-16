package remember

import (
	"bytes"
	"context"

	"testing"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
)

func TestRememberCmd_Basic(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

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
	reg, err := registry.Open(ctx, env.RegistryPath())
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
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	// Use --set-skip-ext to replace defaults entirely
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--set-skip-ext=.log,.tmp", "--skip-hidden=false"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	// Verify config was stored
	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
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
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--add-include-file=.env,.envrc"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)
	if len(rp.Config.IncludeFiles) != 2 {
		t.Errorf("expected 2 include files, got %d", len(rp.Config.IncludeFiles))
	}
}

func TestRememberCmd_NonExistentPath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	nonExistent := env.ConfigDir + "/doesnotexist"

	cmd := createTestCommand()
	cmd.SetArgs([]string{nonExistent})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestRememberCmd_FileInsteadOfDir(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")
	testFile := env.CreateTestFile(testDir, "file.txt", "content")

	cmd := createTestCommand()
	cmd.SetArgs([]string{testFile})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for file path")
	}
}

func TestRememberCmd_DuplicatePath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

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
			env := testutil.NewTestEnv(t)
			testDir := env.CreateTestDir("testproject")

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
			reg, _ := registry.Open(ctx, env.RegistryPath())
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
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	// Remember with no flags - defaults should be applied
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
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
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	// Add a unique extension not in defaults
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--add-skip-ext=.myext"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
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
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	// Set extensions - should replace defaults
	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--set-skip-ext=.only"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("remember command failed: %v", err)
	}

	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
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

func TestMergeUnique(t *testing.T) {
	tests := []struct {
		name      string
		base      []string
		additions []string
		want      []string
	}{
		{
			name:      "empty slices",
			base:      nil,
			additions: nil,
			want:      []string{},
		},
		{
			name:      "only base",
			base:      []string{"a", "b"},
			additions: nil,
			want:      []string{"a", "b"},
		},
		{
			name:      "only additions",
			base:      nil,
			additions: []string{"c", "d"},
			want:      []string{"c", "d"},
		},
		{
			name:      "no overlap",
			base:      []string{"a", "b"},
			additions: []string{"c", "d"},
			want:      []string{"a", "b", "c", "d"},
		},
		{
			name:      "with overlap",
			base:      []string{"a", "b", "c"},
			additions: []string{"b", "c", "d"},
			want:      []string{"a", "b", "c", "d"},
		},
		{
			name:      "duplicates in base",
			base:      []string{"a", "a", "b"},
			additions: []string{"c"},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "duplicates in additions",
			base:      []string{"a"},
			additions: []string{"b", "b", "c", "c"},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "duplicates in both",
			base:      []string{"a", "a", "b"},
			additions: []string{"b", "c", "c"},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "all same values",
			base:      []string{"a", "a"},
			additions: []string{"a", "a"},
			want:      []string{"a"},
		},
		{
			name:      "empty strings",
			base:      []string{"", "a"},
			additions: []string{"b", ""},
			want:      []string{"", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeUnique(tt.base, tt.additions)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d items, got %d", len(tt.want), len(got))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("item %d: expected %q, got %q", i, tt.want[i], v)
				}
			}
		})
	}
}

func TestNormalizeExtensions(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "already with dots",
			input: []string{".log", ".tmp"},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "without dots",
			input: []string{"log", "tmp"},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "mixed",
			input: []string{".log", "tmp"},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "with spaces",
			input: []string{" .log ", " tmp "},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "nil slice",
			input: nil,
			want:  []string{},
		},
		{
			name:  "empty string in slice",
			input: []string{".go", "", ".py"},
			want:  []string{".go", "", ".py"},
		},
		{
			name:  "whitespace only string",
			input: []string{".go", "   ", ".py"},
			want:  []string{".go", "", ".py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeExtensions(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d extensions, got %d", len(tt.want), len(got))
			}
			for i, ext := range got {
				if ext != tt.want[i] {
					t.Errorf("extension %d: expected %q, got %q", i, tt.want[i], ext)
				}
			}
		})
	}
}

// Helper functions

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

func TestHasModificationFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "no flags",
			args: []string{"/tmp/test"},
			want: false,
		},
		{
			name: "add-skip-ext flag",
			args: []string{"/tmp/test", "--add-skip-ext=.log"},
			want: true,
		},
		{
			name: "set-skip-ext flag",
			args: []string{"/tmp/test", "--set-skip-ext=.log"},
			want: true,
		},
		{
			name: "add-skip-dir flag",
			args: []string{"/tmp/test", "--add-skip-dir=vendor"},
			want: true,
		},
		{
			name: "set-skip-dir flag",
			args: []string{"/tmp/test", "--set-skip-dir=vendor"},
			want: true,
		},
		{
			name: "add-skip-file flag",
			args: []string{"/tmp/test", "--add-skip-file=Makefile"},
			want: true,
		},
		{
			name: "set-skip-file flag",
			args: []string{"/tmp/test", "--set-skip-file=Makefile"},
			want: true,
		},
		{
			name: "add-include-ext flag",
			args: []string{"/tmp/test", "--add-include-ext=.go"},
			want: true,
		},
		{
			name: "add-include-dir flag",
			args: []string{"/tmp/test", "--add-include-dir=src"},
			want: true,
		},
		{
			name: "add-include-file flag",
			args: []string{"/tmp/test", "--add-include-file=.env"},
			want: true,
		},
		{
			name: "skip-hidden flag",
			args: []string{"/tmp/test", "--skip-hidden=false"},
			want: true,
		},
		{
			name: "use-vision flag",
			args: []string{"/tmp/test", "--use-vision=true"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestCommand()
			cmd.SetArgs(tt.args)
			cmd.ParseFlags(tt.args)

			got := hasModificationFlags(cmd)
			if got != tt.want {
				t.Errorf("hasModificationFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildUpdatedConfig(t *testing.T) {
	tests := []struct {
		name     string
		existing *registry.PathConfig
		args     []string
		check    func(t *testing.T, cfg *registry.PathConfig)
	}{
		{
			name: "override skip-hidden",
			existing: &registry.PathConfig{
				SkipHidden:     true,
				SkipExtensions: []string{".exe"},
			},
			args: []string{"/tmp/test", "--skip-hidden=false"},
			check: func(t *testing.T, cfg *registry.PathConfig) {
				if cfg.SkipHidden {
					t.Error("expected SkipHidden to be false")
				}
				// Original extension should be preserved
				if len(cfg.SkipExtensions) != 1 || cfg.SkipExtensions[0] != ".exe" {
					t.Error("expected SkipExtensions to be preserved")
				}
			},
		},
		{
			name: "add skip extension to existing",
			existing: &registry.PathConfig{
				SkipExtensions: []string{".exe", ".dll"},
			},
			args: []string{"/tmp/test", "--add-skip-ext=.log"},
			check: func(t *testing.T, cfg *registry.PathConfig) {
				if len(cfg.SkipExtensions) != 3 {
					t.Errorf("expected 3 skip extensions, got %d", len(cfg.SkipExtensions))
				}
				hasLog := false
				hasExe := false
				for _, ext := range cfg.SkipExtensions {
					if ext == ".log" {
						hasLog = true
					}
					if ext == ".exe" {
						hasExe = true
					}
				}
				if !hasLog {
					t.Error("expected .log to be added")
				}
				if !hasExe {
					t.Error("expected .exe to be preserved")
				}
			},
		},
		{
			name: "set skip extension replaces existing",
			existing: &registry.PathConfig{
				SkipExtensions: []string{".exe", ".dll"},
			},
			args: []string{"/tmp/test", "--set-skip-ext=.only"},
			check: func(t *testing.T, cfg *registry.PathConfig) {
				if len(cfg.SkipExtensions) != 1 {
					t.Errorf("expected 1 skip extension, got %d", len(cfg.SkipExtensions))
				}
				if cfg.SkipExtensions[0] != ".only" {
					t.Errorf("expected .only, got %s", cfg.SkipExtensions[0])
				}
			},
		},
		{
			name: "add include file to existing",
			existing: &registry.PathConfig{
				IncludeFiles: []string{".env"},
			},
			args: []string{"/tmp/test", "--add-include-file=.envrc"},
			check: func(t *testing.T, cfg *registry.PathConfig) {
				if len(cfg.IncludeFiles) != 2 {
					t.Errorf("expected 2 include files, got %d", len(cfg.IncludeFiles))
				}
			},
		},
		{
			name: "set use-vision",
			existing: &registry.PathConfig{
				UseVision: nil,
			},
			args: []string{"/tmp/test", "--use-vision=false"},
			check: func(t *testing.T, cfg *registry.PathConfig) {
				if cfg.UseVision == nil {
					t.Fatal("expected UseVision to be set")
				}
				if *cfg.UseVision {
					t.Error("expected UseVision to be false")
				}
			},
		},
		{
			name:     "nil existing config",
			existing: nil,
			args:     []string{"/tmp/test", "--add-skip-ext=.log"},
			check: func(t *testing.T, cfg *registry.PathConfig) {
				if cfg == nil {
					t.Fatal("expected config to not be nil")
				}
				hasLog := false
				for _, ext := range cfg.SkipExtensions {
					if ext == ".log" {
						hasLog = true
					}
				}
				if !hasLog {
					t.Error("expected .log to be added")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestCommand()
			cmd.SetArgs(tt.args)
			cmd.ParseFlags(tt.args)

			// Parse use-vision flag if present
			if useVisionFlag != "" {
				switch useVisionFlag {
				case "true":
					v := true
					rememberUseVision = &v
				case "false":
					v := false
					rememberUseVision = &v
				}
			}

			got := buildUpdatedConfig(cmd, tt.existing)
			tt.check(t, got)
		})
	}
}

func TestRememberCmd_UpdateExistingPath(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	// Remember once with initial config
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir, "--set-skip-ext=.exe"})
	err := cmd1.Execute()
	if err != nil {
		t.Fatalf("first remember failed: %v", err)
	}

	// Verify initial config
	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
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
	reg2, _ := registry.Open(ctx, env.RegistryPath())
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

func TestBuildUpdatedConfig_MultipleAddFlags(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

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
	reg, _ := registry.Open(ctx, env.RegistryPath())
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

func TestBuildUpdatedConfig_AddSkipDir(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

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
	reg, _ := registry.Open(ctx, env.RegistryPath())
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

func TestBuildUpdatedConfig_SetSkipDirReplacesExisting(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

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
	reg, _ := registry.Open(ctx, env.RegistryPath())
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

func TestRememberCmd_UpdateExistingPath_ReWalkWarning(t *testing.T) {
	env := testutil.NewTestEnv(t)
	testDir := env.CreateTestDir("testproject")

	// Remember once
	cmd1 := createTestCommand()
	cmd1.SetArgs([]string{testDir})
	cmd1.Execute()

	// Remember again with modification - daemon isn't running so re-walk will fail
	cmd2 := createTestCommand()
	cmd2.SetArgs([]string{testDir, "--skip-hidden=false"})

	err := cmd2.Execute()
	// Command should succeed even if re-walk fails
	// The warning is printed to stdout via fmt.Printf (visible in test output)
	if err != nil {
		t.Fatalf("update should succeed even if re-walk fails: %v", err)
	}

	// Verify config was actually updated
	ctx := context.Background()
	reg, _ := registry.Open(ctx, env.RegistryPath())
	defer reg.Close()

	rp, _ := reg.GetPath(ctx, testDir)
	if rp.Config.SkipHidden {
		t.Error("expected SkipHidden to be false after update")
	}
}
