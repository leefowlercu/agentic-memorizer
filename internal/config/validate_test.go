package config

import (
	"strings"
	"testing"
)

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
				LogFile:  "~/.agentic-memorizer/mcp.log",
				LogLevel: "info",
			},
			wantErrors: false,
		},
		{
			name: "valid debug log level",
			mcp: MCPConfig{
				LogFile:  "~/.agentic-memorizer/mcp.log",
				LogLevel: "debug",
			},
			wantErrors: false,
		},
		{
			name: "valid warn log level",
			mcp: MCPConfig{
				LogFile:  "~/.agentic-memorizer/mcp.log",
				LogLevel: "warn",
			},
			wantErrors: false,
		},
		{
			name: "valid error log level",
			mcp: MCPConfig{
				LogFile:  "~/.agentic-memorizer/mcp.log",
				LogLevel: "error",
			},
			wantErrors: false,
		},
		{
			name: "invalid log level",
			mcp: MCPConfig{
				LogFile:  "~/.agentic-memorizer/mcp.log",
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
				MemoryRoot: "~/.agentic-memorizer/memory",
				Claude: ClaudeConfig{
					APIKeyEnv:      "ANTHROPIC_API_KEY",
					Model:          "claude-sonnet-4-5-20250929",
					MaxTokens:      1500,
					TimeoutSeconds: 30,
				},
				Output: OutputConfig{
					Format:         "xml",
					ShowRecentDays: 7,
				},
				Analysis: AnalysisConfig{
					CacheDir: "~/.agentic-memorizer/.cache",
				},
				Daemon: DaemonConfig{
					Workers:         3,
					RateLimitPerMin: 20,
					LogFile:         "~/.agentic-memorizer/daemon.log",
					LogLevel:        "info",
				},
				MCP: MCPConfig{
					LogFile:  "~/.agentic-memorizer/mcp.log",
					LogLevel: "info",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid MCP log level",
			cfg: &Config{
				MemoryRoot: "~/.agentic-memorizer/memory",
				Claude: ClaudeConfig{
					APIKeyEnv:      "ANTHROPIC_API_KEY",
					Model:          "claude-sonnet-4-5-20250929",
					MaxTokens:      1500,
					TimeoutSeconds: 30,
				},
				Output: OutputConfig{
					Format:         "xml",
					ShowRecentDays: 7,
				},
				Analysis: AnalysisConfig{
					CacheDir: "~/.agentic-memorizer/.cache",
				},
				Daemon: DaemonConfig{
					Workers:         3,
					RateLimitPerMin: 20,
					LogFile:         "~/.agentic-memorizer/daemon.log",
					LogLevel:        "info",
				},
				MCP: MCPConfig{
					LogFile:  "~/.agentic-memorizer/mcp.log",
					LogLevel: "verbose",
				},
			},
			wantErr:     true,
			errContains: "mcp.log_level",
		},
		{
			name: "unsafe MCP log file path",
			cfg: &Config{
				MemoryRoot: "~/.agentic-memorizer/memory",
				Claude: ClaudeConfig{
					APIKeyEnv:      "ANTHROPIC_API_KEY",
					Model:          "claude-sonnet-4-5-20250929",
					MaxTokens:      1500,
					TimeoutSeconds: 30,
				},
				Output: OutputConfig{
					Format:         "xml",
					ShowRecentDays: 7,
				},
				Analysis: AnalysisConfig{
					CacheDir: "~/.agentic-memorizer/.cache",
				},
				Daemon: DaemonConfig{
					Workers:         3,
					RateLimitPerMin: 20,
					LogFile:         "~/.agentic-memorizer/daemon.log",
					LogLevel:        "info",
				},
				MCP: MCPConfig{
					LogFile:  "~/../tmp/mcp.log",
					LogLevel: "info",
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
