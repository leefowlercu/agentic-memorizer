package initialize

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateNoEmbeddingsConflict(t *testing.T) {
	tests := []struct {
		name           string
		noEmbeddings   bool
		conflictFlag   string
		conflictValue  string
		wantErr        bool
		wantErrContain string
	}{
		{"no conflict when disabled not set", false, "", "", false, ""},
		{"no conflict when no other flags", true, "", "", false, ""},
		{"conflict with embeddings-provider", true, "embeddings-provider", "voyage", true, "cannot combine --no-embeddings with --embeddings-provider"},
		{"conflict with embeddings-model", true, "embeddings-model", "text-embedding-3-large", true, "cannot combine --no-embeddings with --embeddings-model"},
		{"conflict with embeddings-api-key", true, "embeddings-api-key", "test-key", true, "cannot combine --no-embeddings with --embeddings-api-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup command with flags
			cmd := &cobra.Command{}
			cmd.Flags().BoolVar(&initializeNoEmbeddings, "no-embeddings", false, "")
			cmd.Flags().StringVar(&initializeEmbeddingsProvider, "embeddings-provider", "", "")
			cmd.Flags().StringVar(&initializeEmbeddingsModel, "embeddings-model", "", "")
			cmd.Flags().StringVar(&initializeEmbeddingsAPIKey, "embeddings-api-key", "", "")

			if tt.noEmbeddings {
				initializeNoEmbeddings = true
				cmd.Flags().Set("no-embeddings", "true")
			}

			if tt.conflictFlag != "" {
				cmd.Flags().Set(tt.conflictFlag, tt.conflictValue)
			}

			// Test
			err := validateNoEmbeddingsConflict(cmd)

			if tt.wantErr {
				if err == nil {
					t.Error("validateNoEmbeddingsConflict() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("validateNoEmbeddingsConflict() error = %q, want to contain %q", err.Error(), tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("validateNoEmbeddingsConflict() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateNoSemanticConflict(t *testing.T) {
	tests := []struct {
		name           string
		noSemantic     bool
		conflictFlag   string
		conflictValue  string
		wantErr        bool
		wantErrContain string
	}{
		{"no conflict when disabled not set", false, "", "", false, ""},
		{"no conflict when no other flags", true, "", "", false, ""},
		{"conflict with semantic-provider", true, "semantic-provider", "openai", true, "cannot combine --no-semantic with --semantic-provider"},
		{"conflict with semantic-model", true, "semantic-model", "gpt-4o", true, "cannot combine --no-semantic with --semantic-model"},
		{"conflict with semantic-api-key", true, "semantic-api-key", "test-key", true, "cannot combine --no-semantic with --semantic-api-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().BoolVar(&initializeNoSemantic, "no-semantic", false, "")
			cmd.Flags().StringVar(&initializeSemanticProvider, "semantic-provider", "", "")
			cmd.Flags().StringVar(&initializeSemanticModel, "semantic-model", "", "")
			cmd.Flags().StringVar(&initializeSemanticAPIKey, "semantic-api-key", "", "")

			if tt.noSemantic {
				initializeNoSemantic = true
				cmd.Flags().Set("no-semantic", "true")
			}

			if tt.conflictFlag != "" {
				cmd.Flags().Set(tt.conflictFlag, tt.conflictValue)
			}

			err := validateNoSemanticConflict(cmd)

			if tt.wantErr {
				if err == nil {
					t.Error("validateNoSemanticConflict() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("validateNoSemanticConflict() error = %q, want to contain %q", err.Error(), tt.wantErrContain)
				}
			} else if err != nil {
				t.Errorf("validateNoSemanticConflict() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateProvider(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		validProviders map[string]bool
		flagName       string
		wantErr        bool
	}{
		{"valid anthropic", "anthropic", validSemanticProviders, "semantic-provider", false},
		{"valid openai", "openai", validSemanticProviders, "semantic-provider", false},
		{"valid google", "google", validSemanticProviders, "semantic-provider", false},
		{"invalid semantic provider", "invalid", validSemanticProviders, "semantic-provider", true},
		{"valid embeddings openai", "openai", validEmbeddingsProviders, "embeddings-provider", false},
		{"valid embeddings voyage", "voyage", validEmbeddingsProviders, "embeddings-provider", false},
		{"valid embeddings google", "google", validEmbeddingsProviders, "embeddings-provider", false},
		{"invalid embeddings provider", "anthropic", validEmbeddingsProviders, "embeddings-provider", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProvider(tt.provider, tt.validProviders, tt.flagName)

			if tt.wantErr {
				if err == nil {
					t.Error("validateProvider() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("validateProvider() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		flagName string
		wantErr  bool
	}{
		{"valid port 80", 80, "http-port", false},
		{"valid port 443", 443, "http-port", false},
		{"valid port 8080", 8080, "http-port", false},
		{"valid port 65535", 65535, "http-port", false},
		{"valid port 1", 1, "http-port", false},
		{"invalid port 0", 0, "http-port", true},
		{"invalid port negative", -1, "http-port", true},
		{"invalid port too high", 65536, "http-port", true},
		{"invalid port way too high", 99999, "http-port", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port, tt.flagName)

			if tt.wantErr {
				if err == nil {
					t.Error("validatePort() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("validatePort() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateOutputFlag(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		flagSet bool
		wantErr bool
	}{
		{"valid text", "text", true, false},
		{"valid json", "json", true, false},
		{"invalid format", "xml", true, true},
		{"flag not set uses default", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().StringVar(&initializeOutput, "output", "text", "")

			if tt.flagSet {
				initializeOutput = tt.output
				cmd.Flags().Set("output", tt.output)
			}

			err := validateOutputFlag(cmd)

			if tt.wantErr {
				if err == nil {
					t.Error("validateOutputFlag() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("validateOutputFlag() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateRequiredAPIKeys(t *testing.T) {
	tests := []struct {
		name              string
		semanticEnabled   bool
		semanticKey       string
		embeddingsEnabled bool
		embeddingsKey     string
		wantErr           bool
		wantErrContain    string
	}{
		{"all keys present", true, "semantic-key", true, "embeddings-key", false, ""},
		{"missing semantic key", true, "", true, "embeddings-key", true, "semantic API key is required"},
		{"semantic disabled no key needed", false, "", true, "embeddings-key", false, ""},
		{"missing embeddings key when enabled", true, "semantic-key", true, "", true, "embeddings API key is required"},
		{"embeddings disabled no key needed", true, "semantic-key", false, "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := &UnattendedConfig{
				SemanticEnabled:    tt.semanticEnabled,
				SemanticProvider:   "anthropic",
				SemanticAPIKey:     tt.semanticKey,
				EmbeddingsEnabled:  tt.embeddingsEnabled,
				EmbeddingsProvider: "openai",
				EmbeddingsAPIKey:   tt.embeddingsKey,
			}

			err := validateRequiredAPIKeys(resolved)

			if tt.wantErr {
				if err == nil {
					t.Error("validateRequiredAPIKeys() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("validateRequiredAPIKeys() error = %q, want to contain %q", err.Error(), tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("validateRequiredAPIKeys() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateUnattendedFlags(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*cobra.Command)
		wantErr        bool
		wantErrContain string
	}{
		{
			name:    "valid flags",
			setup:   func(cmd *cobra.Command) {},
			wantErr: false,
		},
		{
			name: "invalid semantic provider",
			setup: func(cmd *cobra.Command) {
				initializeSemanticProvider = "invalid"
				cmd.Flags().Set("semantic-provider", "invalid")
			},
			wantErr:        true,
			wantErrContain: "semantic-provider",
		},
		{
			name: "invalid http port",
			setup: func(cmd *cobra.Command) {
				initializeHTTPPort = 99999
				cmd.Flags().Set("http-port", "99999")
			},
			wantErr:        true,
			wantErrContain: "http-port",
		},
		{
			name: "no-semantic conflict",
			setup: func(cmd *cobra.Command) {
				initializeNoSemantic = true
				cmd.Flags().Set("no-semantic", "true")
				cmd.Flags().Set("semantic-provider", "openai")
			},
			wantErr:        true,
			wantErrContain: "cannot combine",
		},
		{
			name: "no-embeddings conflict",
			setup: func(cmd *cobra.Command) {
				initializeNoEmbeddings = true
				cmd.Flags().Set("no-embeddings", "true")
				cmd.Flags().Set("embeddings-provider", "voyage")
			},
			wantErr:        true,
			wantErrContain: "cannot combine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup command with all flags
			cmd := &cobra.Command{}
			cmd.Flags().BoolVar(&initializeNoSemantic, "no-semantic", false, "")
			cmd.Flags().BoolVar(&initializeNoEmbeddings, "no-embeddings", false, "")
			cmd.Flags().StringVar(&initializeSemanticProvider, "semantic-provider", "", "")
			cmd.Flags().StringVar(&initializeSemanticModel, "semantic-model", "", "")
			cmd.Flags().StringVar(&initializeSemanticAPIKey, "semantic-api-key", "", "")
			cmd.Flags().StringVar(&initializeEmbeddingsProvider, "embeddings-provider", "", "")
			cmd.Flags().StringVar(&initializeEmbeddingsModel, "embeddings-model", "", "")
			cmd.Flags().StringVar(&initializeEmbeddingsAPIKey, "embeddings-api-key", "", "")
			cmd.Flags().IntVar(&initializeGraphPort, "graph-port", 0, "")
			cmd.Flags().IntVar(&initializeHTTPPort, "http-port", 0, "")
			cmd.Flags().StringVar(&initializeOutput, "output", "text", "")

			// Reset all flag values
			initializeNoEmbeddings = false
			initializeSemanticProvider = ""
			initializeEmbeddingsProvider = ""
			initializeEmbeddingsModel = ""
			initializeEmbeddingsAPIKey = ""
			initializeGraphPort = 0
			initializeHTTPPort = 0
			initializeOutput = "text"

			tt.setup(cmd)

			err := validateUnattendedFlags(cmd)

			if tt.wantErr {
				if err == nil {
					t.Error("validateUnattendedFlags() expected error, got nil")
				} else if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("validateUnattendedFlags() error = %q, want to contain %q", err.Error(), tt.wantErrContain)
				}
			} else {
				if err != nil {
					t.Errorf("validateUnattendedFlags() unexpected error: %v", err)
				}
			}
		})
	}
}
