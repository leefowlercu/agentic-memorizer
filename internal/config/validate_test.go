package config

import (
	"strings"
	"testing"
)

func TestValidateGraph(t *testing.T) {
	tests := []struct {
		name        string
		graph       GraphConfig
		wantErrors  bool
		errorFields []string
	}{
		{
			name: "valid graph config",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                6379,
				SimilarityThreshold: 0.7,
				MaxSimilarFiles:     10,
			},
			wantErrors: false,
		},
		{
			name: "empty host",
			graph: GraphConfig{
				Host:                "",
				Port:                6379,
				SimilarityThreshold: 0.7,
				MaxSimilarFiles:     10,
			},
			wantErrors:  true,
			errorFields: []string{"graph.host"},
		},
		{
			name: "port zero",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                0,
				SimilarityThreshold: 0.7,
				MaxSimilarFiles:     10,
			},
			wantErrors:  true,
			errorFields: []string{"graph.port"},
		},
		{
			name: "port too high",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                70000,
				SimilarityThreshold: 0.7,
				MaxSimilarFiles:     10,
			},
			wantErrors:  true,
			errorFields: []string{"graph.port"},
		},
		{
			name: "similarity threshold negative",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                6379,
				SimilarityThreshold: -0.5,
				MaxSimilarFiles:     10,
			},
			wantErrors:  true,
			errorFields: []string{"graph.similarity_threshold"},
		},
		{
			name: "similarity threshold too high",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                6379,
				SimilarityThreshold: 1.5,
				MaxSimilarFiles:     10,
			},
			wantErrors:  true,
			errorFields: []string{"graph.similarity_threshold"},
		},
		{
			name: "max similar files zero",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                6379,
				SimilarityThreshold: 0.7,
				MaxSimilarFiles:     0,
			},
			wantErrors:  true,
			errorFields: []string{"graph.max_similar_files"},
		},
		{
			name: "max similar files too high",
			graph: GraphConfig{
				Host:                "localhost",
				Port:                6379,
				SimilarityThreshold: 0.7,
				MaxSimilarFiles:     200,
			},
			wantErrors:  true,
			errorFields: []string{"graph.max_similar_files"},
		},
		{
			name: "multiple errors",
			graph: GraphConfig{
				Host:                "",
				Port:                0,
				SimilarityThreshold: 2.0,
				MaxSimilarFiles:     0,
			},
			wantErrors:  true,
			errorFields: []string{"graph.host", "graph.port", "graph.similarity_threshold", "graph.max_similar_files"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			cfg := &Config{Graph: tt.graph}
			validateGraph(v, cfg)

			if tt.wantErrors && !v.HasErrors() {
				t.Errorf("expected validation errors, got none")
			}

			if !tt.wantErrors && v.HasErrors() {
				t.Errorf("expected no validation errors, got: %v", v.Error())
			}

			if tt.wantErrors {
				// Check that expected error fields are present
				for _, field := range tt.errorFields {
					found := false
					for _, err := range v.Errors {
						if err.Field == field {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error for field %s, but not found in errors: %v", field, v.Errors)
					}
				}
			}
		})
	}
}

func TestValidateMCP(t *testing.T) {
	tests := []struct {
		name        string
		mcp         MCPConfig
		wantErrors  bool
		errorFields []string
	}{
		{
			name: "valid MCP config",
			mcp: MCPConfig{
				LogFile:  "~/.memorizer/mcp.log",
				LogLevel: "info",
			},
			wantErrors: false,
		},
		{
			name: "valid debug log level",
			mcp: MCPConfig{
				LogFile:  "~/.memorizer/mcp.log",
				LogLevel: "debug",
			},
			wantErrors: false,
		},
		{
			name: "valid warn log level",
			mcp: MCPConfig{
				LogFile:  "~/.memorizer/mcp.log",
				LogLevel: "warn",
			},
			wantErrors: false,
		},
		{
			name: "valid error log level",
			mcp: MCPConfig{
				LogFile:  "~/.memorizer/mcp.log",
				LogLevel: "error",
			},
			wantErrors: false,
		},
		{
			name: "invalid log level",
			mcp: MCPConfig{
				LogFile:  "~/.memorizer/mcp.log",
				LogLevel: "trace",
			},
			wantErrors:  true,
			errorFields: []string{"mcp.log_level"},
		},
		{
			name: "empty log file",
			mcp: MCPConfig{
				LogFile:  "",
				LogLevel: "info",
			},
			wantErrors:  true,
			errorFields: []string{"mcp.log_file"},
		},
		{
			name: "log file with parent directory traversal",
			mcp: MCPConfig{
				LogFile:  "~/../etc/passwd",
				LogLevel: "info",
			},
			wantErrors:  true,
			errorFields: []string{"mcp.log_file"},
		},
		{
			name: "multiple errors",
			mcp: MCPConfig{
				LogFile:  "",
				LogLevel: "invalid",
			},
			wantErrors:  true,
			errorFields: []string{"mcp.log_file", "mcp.log_level"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			cfg := &Config{MCP: tt.mcp}
			validateMCP(v, cfg)

			if tt.wantErrors && !v.HasErrors() {
				t.Errorf("expected validation errors, got none")
			}

			if !tt.wantErrors && v.HasErrors() {
				t.Errorf("expected no validation errors, got: %v", v.Error())
			}

			if tt.wantErrors {
				// Check that expected error fields are present
				for _, field := range tt.errorFields {
					found := false
					for _, err := range v.Errors {
						if err.Field == field {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error for field %s, but not found in errors: %v", field, v.Errors)
					}
				}
			}
		})
	}
}

func TestValidateConfig_MCP(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with MCP",
			cfg: &Config{
				MemoryRoot: "~/.memorizer/memory",
				Claude: ClaudeConfig{
					Model:        "claude-sonnet-4-5-20250929",
					MaxTokens:    1500,
					Timeout:      30,
					EnableVision: true,
				},
				Analysis: AnalysisConfig{
					CacheDir: "~/.memorizer/.cache",
				},
				Daemon: DaemonConfig{
					Workers:         3,
					RateLimitPerMin: 20,
					LogFile:         "~/.memorizer/daemon.log",
					LogLevel:        "info",
				},
				MCP: MCPConfig{
					LogFile:  "~/.memorizer/mcp.log",
					LogLevel: "info",
				},
				Graph: GraphConfig{
					Host:                "localhost",
					Port:                6379,
					SimilarityThreshold: 0.7,
					MaxSimilarFiles:     10,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid MCP log level",
			cfg: &Config{
				MemoryRoot: "~/.memorizer/memory",
				Claude: ClaudeConfig{
					Model:        "claude-sonnet-4-5-20250929",
					MaxTokens:    1500,
					Timeout:      30,
					EnableVision: true,
				},
				Analysis: AnalysisConfig{
					CacheDir: "~/.memorizer/.cache",
				},
				Daemon: DaemonConfig{
					Workers:         3,
					RateLimitPerMin: 20,
					LogFile:         "~/.memorizer/daemon.log",
					LogLevel:        "info",
				},
				MCP: MCPConfig{
					LogFile:  "~/.memorizer/mcp.log",
					LogLevel: "verbose",
				},
				Graph: GraphConfig{
					Host:                "localhost",
					Port:                6379,
					SimilarityThreshold: 0.7,
					MaxSimilarFiles:     10,
				},
			},
			wantErr:     true,
			errContains: "mcp.log_level",
		},
		{
			name: "unsafe MCP log file path",
			cfg: &Config{
				MemoryRoot: "~/.memorizer/memory",
				Claude: ClaudeConfig{
					Model:        "claude-sonnet-4-5-20250929",
					MaxTokens:    1500,
					Timeout:      30,
					EnableVision: true,
				},
				Analysis: AnalysisConfig{
					CacheDir: "~/.memorizer/.cache",
				},
				Daemon: DaemonConfig{
					Workers:         3,
					RateLimitPerMin: 20,
					LogFile:         "~/.memorizer/daemon.log",
					LogLevel:        "info",
				},
				MCP: MCPConfig{
					LogFile:  "~/../tmp/mcp.log",
					LogLevel: "info",
				},
				Graph: GraphConfig{
					Host:                "localhost",
					Port:                6379,
					SimilarityThreshold: 0.7,
					MaxSimilarFiles:     10,
				},
			},
			wantErr:     true,
			errContains: "mcp.log_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)

			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}

			if tt.wantErr && err != nil {
				errStr := err.Error()
				if !strings.Contains(errStr, tt.errContains) {
					t.Errorf("expected error to contain %q, got: %v", tt.errContains, errStr)
				}
			}
		})
	}
}

func TestValidateClaude_Timeout(t *testing.T) {
	tests := []struct {
		name       string
		timeout    int
		wantErrors bool
	}{
		{
			name:       "valid timeout minimum",
			timeout:    5,
			wantErrors: false,
		},
		{
			name:       "valid timeout default",
			timeout:    30,
			wantErrors: false,
		},
		{
			name:       "valid timeout maximum",
			timeout:    300,
			wantErrors: false,
		},
		{
			name:       "invalid timeout too low",
			timeout:    4,
			wantErrors: true,
		},
		{
			name:       "invalid timeout zero",
			timeout:    0,
			wantErrors: true,
		},
		{
			name:       "invalid timeout negative",
			timeout:    -1,
			wantErrors: true,
		},
		{
			name:       "invalid timeout too high",
			timeout:    301,
			wantErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			cfg := &Config{
				Claude: ClaudeConfig{
					Model:        "claude-sonnet-4-5-20250929",
					MaxTokens:    1500,
					Timeout:      tt.timeout,
					EnableVision: true,
				},
			}
			validateClaude(v, cfg)

			if tt.wantErrors && !v.HasErrors() {
				t.Errorf("expected validation errors, got none")
			}

			if !tt.wantErrors && v.HasErrors() {
				t.Errorf("expected no validation errors, got: %v", v.Error())
			}
		})
	}
}

func TestValidateEmbeddings_Provider(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		wantErrors bool
	}{
		{
			name:       "valid provider openai",
			provider:   "openai",
			wantErrors: false,
		},
		{
			name:       "valid provider empty (uses default)",
			provider:   "",
			wantErrors: false,
		},
		{
			name:       "invalid provider anthropic",
			provider:   "anthropic",
			wantErrors: true,
		},
		{
			name:       "invalid provider unknown",
			provider:   "unknown-provider",
			wantErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			cfg := &Config{
				Embeddings: EmbeddingsConfig{
					Enabled:    true,
					APIKey:     "test-key",
					Provider:   tt.provider,
					Model:      "text-embedding-3-small",
					Dimensions: 1536,
				},
			}
			validateEmbeddings(v, cfg)

			if tt.wantErrors && !v.HasErrors() {
				t.Errorf("expected validation errors, got none")
			}

			if !tt.wantErrors && v.HasErrors() {
				t.Errorf("expected no validation errors, got: %v", v.Error())
			}
		})
	}
}

func TestValidateEmbeddings_ModelDimensionMatch(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		dimensions int
		wantErrors bool
	}{
		{
			name:       "valid small model dimensions",
			model:      "text-embedding-3-small",
			dimensions: 1536,
			wantErrors: false,
		},
		{
			name:       "valid large model dimensions",
			model:      "text-embedding-3-large",
			dimensions: 3072,
			wantErrors: false,
		},
		{
			name:       "valid ada-002 model dimensions",
			model:      "text-embedding-ada-002",
			dimensions: 1536,
			wantErrors: false,
		},
		{
			name:       "invalid small model wrong dimensions",
			model:      "text-embedding-3-small",
			dimensions: 3072,
			wantErrors: true,
		},
		{
			name:       "invalid large model wrong dimensions",
			model:      "text-embedding-3-large",
			dimensions: 1536,
			wantErrors: true,
		},
		{
			name:       "invalid unknown model",
			model:      "unknown-model",
			dimensions: 1536,
			wantErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			cfg := &Config{
				Embeddings: EmbeddingsConfig{
					Enabled:    true,
					APIKey:     "test-key",
					Provider:   "openai",
					Model:      tt.model,
					Dimensions: tt.dimensions,
				},
			}
			validateEmbeddings(v, cfg)

			if tt.wantErrors && !v.HasErrors() {
				t.Errorf("expected validation errors, got none")
			}

			if !tt.wantErrors && v.HasErrors() {
				t.Errorf("expected no validation errors, got: %v", v.Error())
			}
		})
	}
}
