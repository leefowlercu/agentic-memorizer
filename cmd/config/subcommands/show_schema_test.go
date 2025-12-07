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
		{"table format", "table"},
		{"yaml format", "yaml"},
		{"json format", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			showSchemaFormat = tt.format
			showSchemaAdvancedOnly = false
			showSchemaHardcodedOnly = false

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
	showSchemaHardcodedOnly = false

	err := validateShowSchema(ShowSchemaCmd, []string{})
	if err == nil {
		t.Error("validateShowSchema() expected error for invalid format, got nil")
	}

	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error should mention 'invalid format', got: %v", err)
	}
}

func TestValidateShowSchema_MutuallyExclusiveFlags(t *testing.T) {
	showSchemaFormat = "table"
	showSchemaAdvancedOnly = true
	showSchemaHardcodedOnly = true

	err := validateShowSchema(ShowSchemaCmd, []string{})
	if err == nil {
		t.Error("validateShowSchema() expected error for mutually exclusive flags, got nil")
	}

	if !strings.Contains(err.Error(), "cannot use both") {
		t.Errorf("error should mention 'cannot use both', got: %v", err)
	}
}

func TestRunShowSchema_TableFormat(t *testing.T) {
	showSchemaFormat = "table"
	showSchemaAdvancedOnly = false
	showSchemaHardcodedOnly = false

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
		"CONFIGURABLE SETTINGS",
		"HARDCODED SETTINGS",
		"claude",
		"daemon",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("table output should contain %q", expected)
		}
	}
}

func TestRunShowSchema_YAMLFormat(t *testing.T) {
	showSchemaFormat = "yaml"
	showSchemaAdvancedOnly = false
	showSchemaHardcodedOnly = false

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

	// Check structure
	if _, ok := result["configurable"]; !ok {
		t.Error("YAML output should have 'configurable' key")
	}
	if _, ok := result["hardcoded"]; !ok {
		t.Error("YAML output should have 'hardcoded' key")
	}
}

func TestRunShowSchema_JSONFormat(t *testing.T) {
	showSchemaFormat = "json"
	showSchemaAdvancedOnly = false
	showSchemaHardcodedOnly = false

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

	// Check structure
	if _, ok := result["configurable"]; !ok {
		t.Error("JSON output should have 'configurable' key")
	}
	if _, ok := result["hardcoded"]; !ok {
		t.Error("JSON output should have 'hardcoded' key")
	}
}

func TestRunShowSchema_AdvancedOnly(t *testing.T) {
	showSchemaFormat = "table"
	showSchemaAdvancedOnly = true
	showSchemaHardcodedOnly = false

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

	// Should have configurable section
	if !strings.Contains(output, "CONFIGURABLE SETTINGS") {
		t.Error("advanced-only output should have configurable settings")
	}

	// Should NOT have hardcoded section
	if strings.Contains(output, "HARDCODED SETTINGS") {
		t.Error("advanced-only output should NOT have hardcoded settings")
	}

	// Should show advanced tier fields
	if !strings.Contains(output, "Tier:        advanced") {
		t.Error("advanced-only output should show advanced tier fields")
	}
}

func TestRunShowSchema_HardcodedOnly(t *testing.T) {
	showSchemaFormat = "table"
	showSchemaAdvancedOnly = false
	showSchemaHardcodedOnly = true

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

	// Should NOT have configurable section
	if strings.Contains(output, "CONFIGURABLE SETTINGS") {
		t.Error("hardcoded-only output should NOT have configurable settings")
	}

	// Should have hardcoded section
	if !strings.Contains(output, "HARDCODED SETTINGS") {
		t.Error("hardcoded-only output should have hardcoded settings")
	}

	// Should have hardcoded entries
	if !strings.Contains(output, "Reason:") {
		t.Error("hardcoded-only output should show reasons for hardcoded settings")
	}
}

func TestRunShowSchema_AdvancedOnlyJSON(t *testing.T) {
	showSchemaFormat = "json"
	showSchemaAdvancedOnly = true
	showSchemaHardcodedOnly = false

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

	// Should have configurable
	if _, ok := result["configurable"]; !ok {
		t.Error("JSON output should have 'configurable' key")
	}

	// Should NOT have hardcoded
	if _, ok := result["hardcoded"]; ok {
		t.Error("advanced-only JSON output should NOT have 'hardcoded' key")
	}
}

func TestRunShowSchema_HardcodedOnlyYAML(t *testing.T) {
	showSchemaFormat = "yaml"
	showSchemaAdvancedOnly = false
	showSchemaHardcodedOnly = true

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

	// Verify YAML structure
	var result map[string]any
	if err := yaml.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output is not valid YAML: %v", err)
	}

	// Should NOT have configurable
	if _, ok := result["configurable"]; ok {
		t.Error("hardcoded-only YAML output should NOT have 'configurable' key")
	}

	// Should have hardcoded
	if _, ok := result["hardcoded"]; !ok {
		t.Error("hardcoded-only YAML output should have 'hardcoded' key")
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
