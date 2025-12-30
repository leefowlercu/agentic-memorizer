package subcommands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
	"gopkg.in/yaml.v3"
)

func TestValidateShowSchema_ValidFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"text format", "text"},
		{"yaml format", "yaml"},
		{"json format", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			showSchemaFormat = tt.format
			showSchemaAdvancedOnly = false

			err := validateShowSchema(ShowSchemaCmd, []string{})
			if err != nil {
				t.Errorf("validateShowSchema() returned error for valid format %q: %v", tt.format, err)
			}
		})
	}
}

func TestValidateShowSchema_InvalidFormat(t *testing.T) {
	showSchemaFormat = "invalid"
	showSchemaAdvancedOnly = false

	err := validateShowSchema(ShowSchemaCmd, []string{})
	if err == nil {
		t.Error("validateShowSchema() expected error for invalid format, got nil")
	}

	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error should mention 'invalid format', got: %v", err)
	}
}

func TestRunShowSchema_TextFormat(t *testing.T) {
	showSchemaFormat = "text"
	showSchemaAdvancedOnly = false

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runShowSchema(ShowSchemaCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("runShowSchema() returned error: %v", err)
	}

	// Check for expected content
	expectedStrings := []string{
		"Configuration Schema",
		"semantic",
		"daemon",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("text output should contain %q", expected)
		}
	}

	// Verify "memory:" section appears with proper formatting
	if !strings.Contains(output, "memory:") {
		t.Error("Should display 'memory:' section")
	}

	// Verify config sections have colons
	if !strings.Contains(output, "semantic:") {
		t.Error("Config sections should have colons")
	}

	// Verify indentation: field titles at 2 spaces, field content at 4 spaces
	lines := strings.Split(output, "\n")
	found2SpaceTitle := false
	found4SpaceContent := false
	for _, line := range lines {
		// Check for 2-space indent (field titles like "  api_key:")
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			found2SpaceTitle = true
		}
		// Check for 4-space indent (field content like "    Type:")
		if strings.HasPrefix(line, "    ") && strings.Contains(line, "Type:") {
			found4SpaceContent = true
		}
	}
	if !found2SpaceTitle {
		t.Error("Should find 2-space indented field titles")
	}
	if !found4SpaceContent {
		t.Error("Should find 4-space indented field content")
	}
}

func TestRunShowSchema_YAMLFormat(t *testing.T) {
	showSchemaFormat = "yaml"
	showSchemaAdvancedOnly = false

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runShowSchema(ShowSchemaCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("runShowSchema() returned error: %v", err)
	}

	// Verify it's valid YAML
	var result map[string]any
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid YAML: %v", err)
	}

	// Check structure - should have direct keys for sections
	if _, ok := result["semantic"]; !ok {
		t.Error("YAML output should have 'semantic' key")
	}
	if _, ok := result["daemon"]; !ok {
		t.Error("YAML output should have 'daemon' key")
	}
}

func TestRunShowSchema_JSONFormat(t *testing.T) {
	showSchemaFormat = "json"
	showSchemaAdvancedOnly = false

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runShowSchema(ShowSchemaCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("runShowSchema() returned error: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	// Check structure - should have direct keys for sections
	if _, ok := result["semantic"]; !ok {
		t.Error("JSON output should have 'semantic' key")
	}
	if _, ok := result["daemon"]; !ok {
		t.Error("JSON output should have 'daemon' key")
	}
}

func TestRunShowSchema_AdvancedOnly(t *testing.T) {
	showSchemaFormat = "text"
	showSchemaAdvancedOnly = true

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runShowSchema(ShowSchemaCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("runShowSchema() returned error: %v", err)
	}

	// Should show advanced tier fields
	if !strings.Contains(output, "Tier:        advanced") {
		t.Error("advanced-only output should show advanced tier fields")
	}
}

func TestRunShowSchema_AdvancedOnlyJSON(t *testing.T) {
	showSchemaFormat = "json"
	showSchemaAdvancedOnly = true

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runShowSchema(ShowSchemaCmd, []string{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("runShowSchema() returned error: %v", err)
	}

	// Verify JSON structure
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	// Should have some settings (check for common advanced settings)
	if len(result) == 0 {
		t.Error("JSON output should have some settings")
	}
}

func TestFormatDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"empty string", "", "(empty)"},
		{"non-empty string", "hello", "hello"},
		{"empty slice", []string{}, "[]"},
		{"non-empty slice", []string{"a", "b"}, "[a, b]"},
		{"integer", 42, "42"},
		{"boolean true", true, "true"},
		{"boolean false", false, "false"},
		{"float", 3.14, "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDefault(tt.input)
			if result != tt.expected {
				t.Errorf("formatDefault(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
