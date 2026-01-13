package remember

import (
	"bytes"
	"context"

	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
	"github.com/spf13/cobra"
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

	cmd := createTestCommand()
	cmd.SetArgs([]string{testDir, "--add-skip-ext=.log,.tmp", "--skip-hidden=false"})

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
	cmd.SetArgs([]string{testDir, "--add-include-file=.env,.envrc", "--include-hidden"})

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
	if !rp.Config.IncludeHidden {
		t.Error("expected IncludeHidden to be true")
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
	rememberIncludeHidden = false
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
	cmd.Flags().BoolVar(&rememberIncludeHidden, "include-hidden", false, "")
	cmd.Flags().StringVar(&useVisionFlag, "use-vision", "", "")

	return cmd
}

func boolPtr(b bool) *bool {
	return &b
}
