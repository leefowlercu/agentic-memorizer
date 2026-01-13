package initialize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/steps"
)

func TestInitializeCmd_Usage(t *testing.T) {
	if InitializeCmd.Use != "initialize" {
		t.Errorf("expected Use 'initialize', got '%s'", InitializeCmd.Use)
	}
}

func TestInitializeCmd_Short(t *testing.T) {
	if InitializeCmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

func TestInitializeCmd_Long(t *testing.T) {
	if InitializeCmd.Long == "" {
		t.Error("expected non-empty Long description")
	}
}

func TestInitializeCmd_Example(t *testing.T) {
	if InitializeCmd.Example == "" {
		t.Error("expected non-empty Example")
	}
}

func TestWriteConfig(t *testing.T) {
	// Create temp directory for test config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := viper.New()
	cfg.Set("graph.host", "localhost")
	cfg.Set("graph.port", 6379)
	cfg.Set("semantic.provider", "claude")
	cfg.Set("http.port", 7600)

	cfg.SetConfigFile(configPath)
	cfg.SetConfigType("yaml")

	// Write config to the file
	err := cfg.WriteConfigAs(configPath)
	if err != nil {
		t.Errorf("expected no error writing config, got %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestStepList(t *testing.T) {
	stepList := []any{
		steps.NewFalkorDBStep(),
		steps.NewSemanticProviderStep(),
		steps.NewEmbeddingsStep(),
		steps.NewHTTPPortStep(),
		steps.NewConfirmStep(),
	}

	if len(stepList) != 5 {
		t.Errorf("expected 5 steps, got %d", len(stepList))
	}

	// Verify each step has a title
	for i, step := range stepList {
		s, ok := step.(interface{ Title() string })
		if !ok {
			t.Errorf("step %d does not implement Title()", i)
			continue
		}
		if s.Title() == "" {
			t.Errorf("step %d has empty title", i)
		}
	}
}

func TestInitializeCmd_Flags(t *testing.T) {
	// Verify flags are registered
	unattendedFlag := InitializeCmd.Flags().Lookup("unattended")
	if unattendedFlag == nil {
		t.Error("expected --unattended flag to be registered")
	}

	forceFlag := InitializeCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("expected --force flag to be registered")
	}
}
