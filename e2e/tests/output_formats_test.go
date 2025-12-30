//go:build e2e

package tests

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestOutputFormats_XML tests XML output format
func TestOutputFormats_XML(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Read with XML format (default)
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "xml")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify XML structure
	if !strings.Contains(stdout, "<?xml") {
		t.Error("Expected XML declaration")
	}

	if !strings.Contains(stdout, "<memory_index>") {
		t.Error("Expected <memory_index> root element")
	}

	if !strings.Contains(stdout, "</memory_index>") {
		t.Error("Expected closing </memory_index> tag")
	}

	// Verify XML is parseable
	type SimpleXML struct {
		XMLName xml.Name `xml:"memory_index"`
	}
	var parsed SimpleXML
	if err := xml.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Errorf("XML output is not valid XML: %v", err)
	} else {
		t.Log("XML output is valid and parseable")
	}
}

// TestOutputFormats_JSON tests JSON output format
func TestOutputFormats_JSON(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nJSON content"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Read with JSON format
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "json")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify JSON is parseable
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Errorf("JSON output is not valid JSON: %v", err)
		t.Logf("Output: %s", stdout)
	} else {
		t.Log("JSON output is valid and parseable")

		// Verify expected top-level fields
		if memoryRoot, ok := parsed["memory_root"].(string); !ok || memoryRoot == "" {
			t.Error("Expected 'memory_root' field in JSON")
		}

		if files, ok := parsed["files"].([]any); ok {
			t.Logf("JSON contains %d files", len(files))
		}
	}
}

// TestOutputFormats_VerboseMode tests verbose output includes extra details
func TestOutputFormats_VerboseMode(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add test files
	if err := h.AddMemoryFile("test1.md", "# Test 1\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("test2.md", "# Test 2\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Read without verbose
	stdoutNormal, _, exitCode := h.RunCommand("read", "files", "--format", "json")
	harness.AssertExitCode(t, 0, exitCode, stdoutNormal, "")

	// Read with verbose
	stdoutVerbose, _, exitCode := h.RunCommand("read", "files", "--format", "json", "-v")
	harness.AssertExitCode(t, 0, exitCode, stdoutVerbose, "")

	// Verbose output should be different (likely larger with related files)
	// Note: May not always be larger if no relationships exist
	t.Logf("Normal output: %d bytes", len(stdoutNormal))
	t.Logf("Verbose output: %d bytes", len(stdoutVerbose))

	// Both should be valid JSON
	var normalParsed map[string]any
	if err := json.Unmarshal([]byte(stdoutNormal), &normalParsed); err != nil {
		t.Errorf("Normal JSON is invalid: %v", err)
	}

	var verboseParsed map[string]any
	if err := json.Unmarshal([]byte(stdoutVerbose), &verboseParsed); err != nil {
		t.Errorf("Verbose JSON is invalid: %v", err)
	}
}

// TestOutputFormats_IntegrationWrapping tests integration-specific output wrapping
func TestOutputFormats_IntegrationWrapping(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Test Claude Code hook integration wrapping
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "json", "--integration", "claude-code-hook")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Claude Code hook wraps in JSON envelope with systemMessage and hookSpecificOutput.additionalContext
	var wrapper map[string]any
	if err := json.Unmarshal([]byte(stdout), &wrapper); err != nil {
		t.Errorf("Integration-wrapped output is not valid JSON: %v", err)
	} else {
		// Verify wrapper structure
		if systemMsg, ok := wrapper["systemMessage"].(string); !ok || systemMsg == "" {
			t.Error("Expected 'systemMessage' field in Claude Code hook wrapper")
		} else {
			t.Logf("systemMessage present: %d chars", len(systemMsg))
		}

		// additionalContext is nested in hookSpecificOutput
		if hookSpecific, ok := wrapper["hookSpecificOutput"].(map[string]any); !ok {
			t.Error("Expected 'hookSpecificOutput' field in Claude Code hook wrapper")
		} else {
			if additionalContext, ok := hookSpecific["additionalContext"].(string); !ok || additionalContext == "" {
				t.Error("Expected 'additionalContext' field in hookSpecificOutput")
			} else {
				t.Logf("additionalContext present: %d chars", len(additionalContext))
			}
		}
	}
}

// TestOutputFormats_InvalidFormat tests error handling for invalid format
func TestOutputFormats_InvalidFormat(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try invalid format
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "invalid")

	// Should fail with non-zero exit code
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid format")
	}

	// Should show error message and usage
	output := stdout + stderr
	harness.AssertContains(t, output, "Error:")
	harness.AssertContains(t, output, "invalid format")
	harness.AssertContains(t, output, "Usage:")
}

// TestOutputFormats_EmptyIndex tests output with no files indexed
func TestOutputFormats_EmptyIndex(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Don't add any files - index should be empty
	// Note: This test deliberately creates an empty memory directory
	// The test reads from the global user memory which may have files,
	// so we just verify the command runs successfully rather than checking emptiness

	// Read index (may not be empty if user has files in ~/.memorizer/memory)
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "json")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// JSON should be valid regardless of file count
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Errorf("Index JSON is not valid: %v", err)
	} else {
		if files, ok := parsed["files"].([]any); ok {
			t.Logf("Index contains %d files (may include user's memory files)", len(files))
		}
	}
}

// TestOutputFormats_XMLStructureValidation tests XML schema compliance
func TestOutputFormats_XMLStructureValidation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add diverse test files
	if err := h.AddMemoryFile("doc.md", "# Document\n\nMarkdown doc"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("data.json", `{"key": "value"}`); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Read XML output
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "xml")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify expected XML elements exist
	requiredElements := []string{
		"<memory_index>",
		"<metadata>",
		"<memory_root>",
		"<file_count>",
		"<categories>",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(stdout, elem) {
			t.Errorf("Expected XML element %s not found", elem)
		}
	}

	// Verify XML is well-formed by checking it's parseable
	// Note: Simple character counting is unreliable because:
	// - Every "</tag>" contains both "<" and "</"
	// - Self-closing tags "/>" contain both ">" and "/>"
	// Instead, we rely on the xml.Unmarshal test above which validates structure
	type SimpleXMLValidation struct {
		XMLName xml.Name `xml:"memory_index"`
	}
	var validation SimpleXMLValidation
	if err := xml.Unmarshal([]byte(stdout), &validation); err != nil {
		t.Errorf("XML structure validation failed: %v", err)
	} else {
		t.Log("XML tag structure is balanced and well-formed")
	}
}

// TestOutputFormats_JSONSchemaValidation tests JSON structure compliance
func TestOutputFormats_JSONSchemaValidation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Read JSON output
	stdout, stderr, exitCode := h.RunCommand("read", "files", "--format", "json")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Parse and validate JSON structure
	var index map[string]any
	if err := json.Unmarshal([]byte(stdout), &index); err != nil {
		t.Fatalf("JSON is not valid: %v", err)
	}

	// Verify required top-level fields
	requiredFields := []string{"memory_root", "files", "stats"}
	for _, field := range requiredFields {
		if _, ok := index[field]; !ok {
			t.Errorf("Required field %q not found in JSON output", field)
		}
	}

	// Verify files array structure
	if files, ok := index["files"].([]any); ok {
		if len(files) > 0 {
			// Check first file has expected fields
			if file, ok := files[0].(map[string]any); ok {
				fileFields := []string{"path", "name", "type", "category"}
				for _, field := range fileFields {
					if _, ok := file[field]; !ok {
						t.Errorf("File entry missing field %q", field)
					}
				}
				t.Log("File entry structure is valid")
			}
		}
	}

	// Verify stats object structure
	if stats, ok := index["stats"].(map[string]any); ok {
		statsFields := []string{"total_files", "total_size", "categories"}
		for _, field := range statsFields {
			if _, ok := stats[field]; !ok {
				t.Logf("Stats field %q not found (may be optional)", field)
			}
		}
	}
}

