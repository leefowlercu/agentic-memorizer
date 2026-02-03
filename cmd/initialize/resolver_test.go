package initialize

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetDefaultSemanticModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", "claude-sonnet-4-5-20250929"},
		{"openai", "gpt-4o"},
		{"google", "gemini-2.0-flash"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := getDefaultSemanticModel(tt.provider)
			if got != tt.want {
				t.Errorf("getDefaultSemanticModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGetDefaultEmbeddingsModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "text-embedding-3-large"},
		{"voyage", "voyage-3-large"},
		{"google", "text-embedding-004"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := getDefaultEmbeddingsModel(tt.provider)
			if got != tt.want {
				t.Errorf("getDefaultEmbeddingsModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGetEmbeddingsDimensions(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"text-embedding-3-large", 3072},
		{"text-embedding-3-small", 1536},
		{"text-embedding-004", 768},
		{"voyage-3-large", 1024},
		{"voyage-3", 1024},
		{"voyage-code-3", 1024},
		{"unknown-model", 1536}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := getEmbeddingsDimensions(tt.model)
			if got != tt.want {
				t.Errorf("getEmbeddingsDimensions(%q) = %d, want %d", tt.model, got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		want     int
	}{
		{"valid int", "TEST_INT", "8080", 8080},
		{"zero", "TEST_INT", "0", 0},
		{"empty", "TEST_INT", "", 0},
		{"invalid", "TEST_INT", "not-a-number", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			got := getEnvInt(tt.envKey)
			if got != tt.want {
				t.Errorf("getEnvInt(%q) = %d, want %d", tt.envKey, got, tt.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		want     bool
	}{
		{"true", "TEST_BOOL", "true", true},
		{"false", "TEST_BOOL", "false", false},
		{"1", "TEST_BOOL", "1", true},
		{"0", "TEST_BOOL", "0", false},
		{"empty", "TEST_BOOL", "", false},
		{"invalid", "TEST_BOOL", "not-a-bool", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			} else {
				os.Unsetenv(tt.envKey)
			}

			got := getEnvBool(tt.envKey)
			if got != tt.want {
				t.Errorf("getEnvBool(%q) = %v, want %v", tt.envKey, got, tt.want)
			}
		})
	}
}

func TestResolveGraphHost(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		flagSet   bool
		envValue  string
		want      string
	}{
		{"flag takes precedence", "flag-host", true, "env-host", "flag-host"},
		{"env fallback", "", false, "env-host", "env-host"},
		{"default", "", false, "", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			cmd.Flags().StringVar(&initializeGraphHost, "graph-host", "", "")

			if tt.flagSet {
				initializeGraphHost = tt.flagValue
				cmd.Flags().Set("graph-host", tt.flagValue)
			} else {
				initializeGraphHost = ""
			}

			if tt.envValue != "" {
				os.Setenv("MEMORIZER_GRAPH_HOST", tt.envValue)
				defer os.Unsetenv("MEMORIZER_GRAPH_HOST")
			} else {
				os.Unsetenv("MEMORIZER_GRAPH_HOST")
			}

			// Test
			got := resolveGraphHost(cmd)
			if got != tt.want {
				t.Errorf("resolveGraphHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveGraphPort(t *testing.T) {
	tests := []struct {
		name      string
		flagValue int
		flagSet   bool
		envValue  string
		want      int
	}{
		{"flag takes precedence", 6380, true, "6381", 6380},
		{"env fallback", 0, false, "6381", 6381},
		{"default", 0, false, "", 6379},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			cmd.Flags().IntVar(&initializeGraphPort, "graph-port", 0, "")

			if tt.flagSet {
				initializeGraphPort = tt.flagValue
				cmd.Flags().Set("graph-port", "6380")
			} else {
				initializeGraphPort = 0
			}

			if tt.envValue != "" {
				os.Setenv("MEMORIZER_GRAPH_PORT", tt.envValue)
				defer os.Unsetenv("MEMORIZER_GRAPH_PORT")
			} else {
				os.Unsetenv("MEMORIZER_GRAPH_PORT")
			}

			// Test
			got := resolveGraphPort(cmd)
			if got != tt.want {
				t.Errorf("resolveGraphPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveSemanticProvider(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		flagSet   bool
		envValue  string
		want      string
	}{
		{"flag takes precedence", "openai", true, "google", "openai"},
		{"env fallback", "", false, "google", "google"},
		{"default", "", false, "", "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			cmd.Flags().StringVar(&initializeSemanticProvider, "semantic-provider", "", "")

			if tt.flagSet {
				initializeSemanticProvider = tt.flagValue
				cmd.Flags().Set("semantic-provider", tt.flagValue)
			} else {
				initializeSemanticProvider = ""
			}

			if tt.envValue != "" {
				os.Setenv("MEMORIZER_SEMANTIC_PROVIDER", tt.envValue)
				defer os.Unsetenv("MEMORIZER_SEMANTIC_PROVIDER")
			} else {
				os.Unsetenv("MEMORIZER_SEMANTIC_PROVIDER")
			}

			// Test
			got := resolveSemanticProvider(cmd)
			if got != tt.want {
				t.Errorf("resolveSemanticProvider() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSemanticAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		flagValue  string
		flagSet    bool
		envValue   string
		wantKey    string
		wantSource string
	}{
		{"flag takes precedence", "anthropic", "flag-key", true, "env-key", "flag-key", "flag"},
		{"env fallback anthropic", "anthropic", "", false, "env-key", "env-key", "ANTHROPIC_API_KEY"},
		{"env fallback openai", "openai", "", false, "env-key", "env-key", "OPENAI_API_KEY"},
		{"env fallback google", "google", "", false, "env-key", "env-key", "GOOGLE_API_KEY"},
		{"no key", "anthropic", "", false, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			cmd.Flags().StringVar(&initializeSemanticAPIKey, "semantic-api-key", "", "")

			if tt.flagSet {
				initializeSemanticAPIKey = tt.flagValue
				cmd.Flags().Set("semantic-api-key", tt.flagValue)
			} else {
				initializeSemanticAPIKey = ""
			}

			// Clear all provider env vars
			os.Unsetenv("ANTHROPIC_API_KEY")
			os.Unsetenv("OPENAI_API_KEY")
			os.Unsetenv("GOOGLE_API_KEY")

			if tt.envValue != "" {
				envVar := semanticProviderEnvVars[tt.provider]
				os.Setenv(envVar, tt.envValue)
				defer os.Unsetenv(envVar)
			}

			// Test
			gotKey, gotSource := resolveSemanticAPIKey(cmd, tt.provider)
			if gotKey != tt.wantKey {
				t.Errorf("resolveSemanticAPIKey() key = %q, want %q", gotKey, tt.wantKey)
			}
			if gotSource != tt.wantSource {
				t.Errorf("resolveSemanticAPIKey() source = %q, want %q", gotSource, tt.wantSource)
			}
		})
	}
}

func TestResolveEmbeddingsEnabled(t *testing.T) {
	tests := []struct {
		name      string
		flagValue bool
		flagSet   bool
		envValue  string
		want      bool
	}{
		{"flag disables", true, true, "", false},
		{"env false", false, false, "false", false},
		{"env true", false, false, "true", true},
		{"default enabled", false, false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			cmd.Flags().BoolVar(&initializeNoEmbeddings, "no-embeddings", false, "")

			if tt.flagSet {
				initializeNoEmbeddings = tt.flagValue
				cmd.Flags().Set("no-embeddings", "true")
			} else {
				initializeNoEmbeddings = false
			}

			if tt.envValue != "" {
				os.Setenv("MEMORIZER_EMBEDDINGS_ENABLED", tt.envValue)
				defer os.Unsetenv("MEMORIZER_EMBEDDINGS_ENABLED")
			} else {
				os.Unsetenv("MEMORIZER_EMBEDDINGS_ENABLED")
			}

			// Test
			got := resolveEmbeddingsEnabled(cmd)
			if got != tt.want {
				t.Errorf("resolveEmbeddingsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveSemanticEnabled(t *testing.T) {
	tests := []struct {
		name      string
		flagValue bool
		flagSet   bool
		envValue  string
		want      bool
	}{
		{"flag disables", true, true, "", false},
		{"env false", false, false, "false", false},
		{"env true", false, false, "true", true},
		{"default enabled", false, false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().BoolVar(&initializeNoSemantic, "no-semantic", false, "")

			if tt.flagSet {
				initializeNoSemantic = tt.flagValue
				cmd.Flags().Set("no-semantic", "true")
			} else {
				initializeNoSemantic = false
			}

			if tt.envValue != "" {
				os.Setenv("MEMORIZER_SEMANTIC_ENABLED", tt.envValue)
				defer os.Unsetenv("MEMORIZER_SEMANTIC_ENABLED")
			} else {
				os.Unsetenv("MEMORIZER_SEMANTIC_ENABLED")
			}

			got := resolveSemanticEnabled(cmd)
			if got != tt.want {
				t.Errorf("resolveSemanticEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveHTTPPort(t *testing.T) {
	tests := []struct {
		name      string
		flagValue int
		flagSet   bool
		envValue  string
		want      int
	}{
		{"flag takes precedence", 8080, true, "9090", 8080},
		{"env fallback", 0, false, "9090", 9090},
		{"default", 0, false, "", 7600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cmd := &cobra.Command{}
			cmd.Flags().IntVar(&initializeHTTPPort, "http-port", 0, "")

			if tt.flagSet {
				initializeHTTPPort = tt.flagValue
				cmd.Flags().Set("http-port", "8080")
			} else {
				initializeHTTPPort = 0
			}

			if tt.envValue != "" {
				os.Setenv("MEMORIZER_DAEMON_HTTP_PORT", tt.envValue)
				defer os.Unsetenv("MEMORIZER_DAEMON_HTTP_PORT")
			} else {
				os.Unsetenv("MEMORIZER_DAEMON_HTTP_PORT")
			}

			// Test
			got := resolveHTTPPort(cmd)
			if got != tt.want {
				t.Errorf("resolveHTTPPort() = %d, want %d", got, tt.want)
			}
		})
	}
}
