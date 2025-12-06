//go:build !integration

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetAppDir(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantError bool
		validate  func(t *testing.T, result string)
	}{
		{
			name:      "default behavior (no env var set)",
			envValue:  "",
			wantError: false,
			validate: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, AppDirName) {
					t.Errorf("expected path to end with %q, got %q", AppDirName, result)
				}
				home, _ := os.UserHomeDir()
				expected := filepath.Join(home, AppDirName)
				if result != expected {
					t.Errorf("expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "custom app dir from env var",
			envValue:  "/tmp/test-app",
			wantError: false,
			validate: func(t *testing.T, result string) {
				if result != "/tmp/test-app" {
					t.Errorf("expected /tmp/test-app, got %q", result)
				}
			},
		},
		{
			name:      "app dir with tilde expansion",
			envValue:  "~/custom-app-dir",
			wantError: false,
			validate: func(t *testing.T, result string) {
				if strings.HasPrefix(result, "~") {
					t.Error("expected tilde to be expanded, but path still contains ~")
				}
				home, _ := os.UserHomeDir()
				expected := filepath.Join(home, "custom-app-dir")
				if result != expected {
					t.Errorf("expected %q, got %q", expected, result)
				}
			},
		},
		{
			name:      "invalid path (contains /../ traversal)",
			envValue:  "/tmp/../../../etc/passwd",
			wantError: true,
			validate:  nil,
		},
		{
			name:      "valid nested path",
			envValue:  "/tmp/test/nested/app",
			wantError: false,
			validate: func(t *testing.T, result string) {
				if result != "/tmp/test/nested/app" {
					t.Errorf("expected /tmp/test/nested/app, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set or unset environment variable
			if tt.envValue != "" {
				t.Setenv("MEMORIZER_APP_DIR", tt.envValue)
			} else {
				// Ensure env var is not set
				os.Unsetenv("MEMORIZER_APP_DIR")
			}

			result, err := GetAppDir()

			if tt.wantError {
				if err == nil {
					t.Errorf("GetAppDir() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetAppDir() unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestGetAppDir_ConcurrentAccess(t *testing.T) {
	// Test that GetAppDir() is safe for concurrent access
	// This is important since it's called during initialization
	const numGoroutines = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			_, err := GetAppDir()
			if err != nil {
				t.Errorf("concurrent GetAppDir() call failed: %v", err)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestGetPIDPath(t *testing.T) {
	tests := []struct {
		name      string
		appDirEnv string
		wantError bool
		validate  func(t *testing.T, result string)
	}{
		{
			name:      "default PID path",
			appDirEnv: "",
			wantError: false,
			validate: func(t *testing.T, result string) {
				if !strings.HasSuffix(result, DaemonPIDFile) {
					t.Errorf("expected path to end with %q, got %q", DaemonPIDFile, result)
				}
			},
		},
		{
			name:      "custom app dir PID path",
			appDirEnv: "/tmp/test-app",
			wantError: false,
			validate: func(t *testing.T, result string) {
				expected := filepath.Join("/tmp/test-app", DaemonPIDFile)
				if result != expected {
					t.Errorf("expected %q, got %q", expected, result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.appDirEnv != "" {
				t.Setenv("MEMORIZER_APP_DIR", tt.appDirEnv)
			} else {
				os.Unsetenv("MEMORIZER_APP_DIR")
			}

			result, err := GetPIDPath()

			if tt.wantError {
				if err == nil {
					t.Error("GetPIDPath() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetPIDPath() unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
