//go:build !integration

package config

import (
	"strings"
	"testing"
)

func TestValidateReload(t *testing.T) {
	tests := []struct {
		name       string
		oldCfg     *Config
		newCfg     *Config
		wantError  bool
		errorField string // The field that should be mentioned in error
	}{
		{
			name: "no changes",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon:   DaemonConfig{LogFile: "/test/daemon.log"},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon:   DaemonConfig{LogFile: "/test/daemon.log"},
			},
			wantError: false,
		},
		{
			name: "memory_root changed",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/old/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon:   DaemonConfig{LogFile: "/test/daemon.log"},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/new/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon:   DaemonConfig{LogFile: "/test/daemon.log"},
			},
			wantError:  true,
			errorField: "memory.root",
		},
		{
			name: "cache_dir changed",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/old/cache"},
				Daemon:   DaemonConfig{LogFile: "/test/daemon.log"},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/new/cache"},
				Daemon:   DaemonConfig{LogFile: "/test/daemon.log"},
			},
			wantError:  true,
			errorField: "semantic.cache_dir",
		},
		{
			name: "log_file changed",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon:   DaemonConfig{LogFile: "/old/daemon.log"},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon:   DaemonConfig{LogFile: "/new/daemon.log"},
			},
			wantError:  true,
			errorField: "daemon.log_file",
		},
		{
			name: "multiple immutable fields changed",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/old/memory"},
				Semantic: SemanticConfig{CacheDir: "/old/cache"},
				Daemon:   DaemonConfig{LogFile: "/old/daemon.log"},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/new/memory"},
				Semantic: SemanticConfig{CacheDir: "/new/cache"},
				Daemon:   DaemonConfig{LogFile: "/new/daemon.log"},
			},
			wantError:  true,
			errorField: "memory.root", // Should mention at least one field
		},
		{
			name: "workers changed (hot-reloadable)",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile: "/test/daemon.log",
					Workers: 3,
				},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile: "/test/daemon.log",
					Workers: 8,
				},
			},
			wantError: false,
		},
		{
			name: "rate_limit changed (hot-reloadable)",
			oldCfg: &Config{
				Memory: MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{
					CacheDir:        "/test/cache",
					RateLimitPerMin: 20,
				},
				Daemon: DaemonConfig{
					LogFile: "/test/daemon.log",
				},
			},
			newCfg: &Config{
				Memory: MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{
					CacheDir:        "/test/cache",
					RateLimitPerMin: 40,
				},
				Daemon: DaemonConfig{
					LogFile: "/test/daemon.log",
				},
			},
			wantError: false,
		},
		{
			name: "log_level changed (hot-reloadable)",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:  "/test/daemon.log",
					LogLevel: "info",
				},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:  "/test/daemon.log",
					LogLevel: "debug",
				},
			},
			wantError: false,
		},
		{
			name: "semantic api settings changed (hot-reloadable)",
			oldCfg: &Config{
				Memory: MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{
					CacheDir:  "/test/cache",
					APIKey:    "old-key",
					Model:     "claude-3-sonnet",
					MaxTokens: 1000,
				},
				Daemon: DaemonConfig{LogFile: "/test/daemon.log"},
			},
			newCfg: &Config{
				Memory: MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{
					CacheDir:  "/test/cache",
					APIKey:    "new-key",
					Model:     "claude-3-opus",
					MaxTokens: 2000,
				},
				Daemon: DaemonConfig{LogFile: "/test/daemon.log"},
			},
			wantError: false,
		},
		{
			name: "debounce interval changed (hot-reloadable)",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:    "/test/daemon.log",
					DebounceMs: 500,
				},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:    "/test/daemon.log",
					DebounceMs: 1000,
				},
			},
			wantError: false,
		},
		{
			name: "rebuild interval changed (hot-reloadable)",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:                    "/test/daemon.log",
					FullRebuildIntervalMinutes: 60,
				},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:                    "/test/daemon.log",
					FullRebuildIntervalMinutes: 120,
				},
			},
			wantError: false,
		},
		{
			name: "http port changed (hot-reloadable)",
			oldCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:  "/test/daemon.log",
					HTTPPort: 8080,
				},
			},
			newCfg: &Config{
				Memory:   MemoryConfig{Root: "/test/memory"},
				Semantic: SemanticConfig{CacheDir: "/test/cache"},
				Daemon: DaemonConfig{
					LogFile:  "/test/daemon.log",
					HTTPPort: 8081,
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReload(tt.oldCfg, tt.newCfg)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateReload() expected error but got nil")
					return
				}
				if tt.errorField != "" && !strings.Contains(err.Error(), tt.errorField) {
					t.Errorf("ValidateReload() error should mention %q, got: %v", tt.errorField, err)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateReload() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateReload_ErrorAccumulation(t *testing.T) {
	// Test that multiple immutable field changes are all reported
	oldCfg := &Config{
		Memory:   MemoryConfig{Root: "/old/memory"},
		Semantic: SemanticConfig{CacheDir: "/old/cache"},
		Daemon:   DaemonConfig{LogFile: "/old/daemon.log"},
	}

	newCfg := &Config{
		Memory:   MemoryConfig{Root: "/new/memory"},
		Semantic: SemanticConfig{CacheDir: "/new/cache"},
		Daemon:   DaemonConfig{LogFile: "/new/daemon.log"},
	}

	err := ValidateReload(oldCfg, newCfg)

	if err == nil {
		t.Fatal("ValidateReload() expected error for multiple immutable changes")
	}

	errMsg := err.Error()

	// All three immutable fields should be mentioned
	if !strings.Contains(errMsg, "memory.root") {
		t.Error("Error should mention memory.root change")
	}
	if !strings.Contains(errMsg, "cache_dir") {
		t.Error("Error should mention cache_dir change")
	}
	if !strings.Contains(errMsg, "log_file") {
		t.Error("Error should mention log_file change")
	}
}
