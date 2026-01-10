package version

import (
	"bytes"
	"strings"
	"testing"
)

// T014: Tests for version command output format
func TestVersionCommandOutput(t *testing.T) {
	// Create a buffer to capture output
	buf := new(bytes.Buffer)
	VersionCmd.SetOut(buf)

	// Execute the command
	err := VersionCmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()

	// Verify output contains required labels
	requiredLabels := []string{"Version:", "Git Commit:", "Build Date:"}
	for _, label := range requiredLabels {
		if !strings.Contains(output, label) {
			t.Errorf("version output missing label %q", label)
		}
	}
}

func TestVersionCommandOutputFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	VersionCmd.SetOut(buf)

	err := VersionCmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have exactly 3 lines
	if len(lines) != 3 {
		t.Errorf("version output has %d lines, expected 3", len(lines))
	}

	// Each line should have a label and value
	for i, line := range lines {
		if !strings.Contains(line, ":") {
			t.Errorf("line %d missing colon separator: %q", i+1, line)
		}
	}
}
