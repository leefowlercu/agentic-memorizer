package config

import (
	"strings"
	"testing"
)

func TestValidate_ValidConfig_ReturnsNil(t *testing.T) {
	cfg := NewDefaultConfig()
	err := Validate(&cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil for valid config", err)
	}
}

func TestValidate_InvalidHTTPPort_ReturnsError(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"port too high", 65536},
		{"way too high", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.Daemon.HTTPPort = tt.port

			err := Validate(&cfg)
			if err == nil {
				t.Errorf("Validate() expected error for port %d", tt.port)
			}

			if !IsValidationError(err) {
				t.Errorf("expected validation error, got %T", err)
			}
		})
	}
}

func TestValidate_EmptyHTTPBind_ReturnsError(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Daemon.HTTPBind = ""

	err := Validate(&cfg)
	if err == nil {
		t.Error("Validate() expected error for empty http_bind")
	}
}

func TestValidate_InvalidShutdownTimeout_ReturnsError(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Daemon.ShutdownTimeout = 0

	err := Validate(&cfg)
	if err == nil {
		t.Error("Validate() expected error for zero shutdown_timeout")
	}
}

func TestValidate_InvalidGraphPort_ReturnsError(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Graph.Port = 0

	err := Validate(&cfg)
	if err == nil {
		t.Error("Validate() expected error for zero graph port")
	}
}

func TestValidate_InvalidSemanticProvider_ReturnsError(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Semantic.Provider = "invalid"

	err := Validate(&cfg)
	if err == nil {
		t.Error("Validate() expected error for invalid semantic provider")
	}
}

func TestValidate_ValidSemanticProviders(t *testing.T) {
	providers := []string{"anthropic", "openai", "google"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.Semantic.Provider = provider

			err := Validate(&cfg)
			if err != nil {
				t.Errorf("Validate() error = %v for valid provider %q", err, provider)
			}
		})
	}
}

func TestValidate_InvalidEmbeddingsProvider_WhenEnabled_ReturnsError(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Embeddings.Enabled = true
	cfg.Embeddings.Provider = "invalid"

	err := Validate(&cfg)
	if err == nil {
		t.Error("Validate() expected error for invalid embeddings provider when enabled")
	}
}

func TestValidate_InvalidEmbeddingsProvider_WhenDisabled_ReturnsNil(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Embeddings.Enabled = false
	cfg.Embeddings.Provider = "invalid"

	err := Validate(&cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, expected nil when embeddings disabled", err)
	}
}

func TestValidate_ValidEmbeddingsProviders(t *testing.T) {
	providers := []string{"openai", "google"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.Embeddings.Enabled = true
			cfg.Embeddings.Provider = provider

			err := Validate(&cfg)
			if err != nil {
				t.Errorf("Validate() error = %v for valid provider %q", err, provider)
			}
		})
	}
}

func TestValidate_MultipleErrors_ReturnsAllErrors(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Daemon.HTTPPort = 0
	cfg.Daemon.HTTPBind = ""
	cfg.Graph.Port = 0

	err := Validate(&cfg)
	if err == nil {
		t.Fatal("Validate() expected error for multiple invalid fields")
	}

	verrs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	if len(verrs) < 3 {
		t.Errorf("expected at least 3 validation errors, got %d", len(verrs))
	}
}

func TestValidationError_Error_FormatsCorrectly(t *testing.T) {
	err := ValidationError{
		Field:   "daemon.http_port",
		Message: "must be between 1 and 65535",
	}

	want := "daemon.http_port: must be between 1 and 65535"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestValidationErrors_Error_FormatsMultiple(t *testing.T) {
	errs := ValidationErrors{
		{Field: "field1", Message: "error1"},
		{Field: "field2", Message: "error2"},
	}

	got := errs.Error()
	if got == "" {
		t.Error("Error() returned empty string for multiple errors")
	}

	// Should contain both errors
	if !strings.Contains(got, "field1") || !strings.Contains(got, "error1") {
		t.Error("Error() missing first error")
	}
	if !strings.Contains(got, "field2") || !strings.Contains(got, "error2") {
		t.Error("Error() missing second error")
	}
}

func TestValidationErrors_Error_SingleError_ReturnsSimpleFormat(t *testing.T) {
	errs := ValidationErrors{
		{Field: "field1", Message: "error1"},
	}

	got := errs.Error()
	want := "field1: error1"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestValidationErrors_Error_Empty_ReturnsEmptyString(t *testing.T) {
	errs := ValidationErrors{}

	if got := errs.Error(); got != "" {
		t.Errorf("Error() = %q, want empty string", got)
	}
}

func TestIsValidationError_WithValidationError_ReturnsTrue(t *testing.T) {
	err := ValidationError{Field: "test", Message: "error"}
	if !IsValidationError(err) {
		t.Error("IsValidationError() = false, want true for ValidationError")
	}
}

func TestIsValidationError_WithValidationErrors_ReturnsTrue(t *testing.T) {
	err := ValidationErrors{{Field: "test", Message: "error"}}
	if !IsValidationError(err) {
		t.Error("IsValidationError() = false, want true for ValidationErrors")
	}
}

func TestIsValidationError_WithOtherError_ReturnsFalse(t *testing.T) {
	err := &testError{}
	if IsValidationError(err) {
		t.Error("IsValidationError() = true, want false for other error types")
	}
}

// Helper types for tests
type testError struct{}

func (e *testError) Error() string { return "test error" }
