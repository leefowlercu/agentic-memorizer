package graph

import (
	"testing"
)

func TestNormalizeEntityName(t *testing.T) {
	d := NewDisambiguation(nil, nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic normalization
		{"lowercase", "Terraform", "terraform"},
		{"trim whitespace", "  kubernetes  ", "kubernetes"},
		{"multiple spaces", "hello   world", "hello world"},

		// Alias resolution
		{"tf alias", "tf", "terraform"},
		{"TF uppercase", "TF", "terraform"},
		{"k8s alias", "k8s", "kubernetes"},
		{"K8S uppercase", "K8S", "kubernetes"},
		{"js alias", "js", "javascript"},
		{"ts alias", "ts", "typescript"},
		{"py alias", "py", "python"},
		{"pg alias", "pg", "postgresql"},
		{"postgres alias", "postgres", "postgresql"},
		{"mongo alias", "mongo", "mongodb"},
		{"node alias", "node", "nodejs"},
		{"aws alias", "aws", "amazon web services"},
		{"gcp alias", "gcp", "google cloud platform"},
		{"azure alias", "azure", "microsoft azure"},
		{"mcp alias", "mcp", "model context protocol"},

		// Non-alias terms pass through
		{"no alias", "docker", "docker"},
		{"no alias complex", "FalkorDB", "falkordb"},
		{"phrase", "Claude Code", "claude code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.NormalizeEntityName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeEntityName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetCanonicalName(t *testing.T) {
	d := NewDisambiguation(nil, nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"tf", "terraform"},
		{"Terraform", "terraform"},
		{"docker", "docker"},
		{"Docker", "docker"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := d.GetCanonicalName(tt.input)
			if result != tt.expected {
				t.Errorf("GetCanonicalName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAddAlias(t *testing.T) {
	d := NewDisambiguation(nil, nil)

	// Add a custom alias
	d.AddAlias("hashi", "hashicorp")

	result := d.GetCanonicalName("hashi")
	if result != "hashicorp" {
		t.Errorf("GetCanonicalName(\"hashi\") after AddAlias = %q, want \"hashicorp\"", result)
	}

	// Verify it works with different cases
	result = d.GetCanonicalName("HASHI")
	if result != "hashicorp" {
		t.Errorf("GetCanonicalName(\"HASHI\") after AddAlias = %q, want \"hashicorp\"", result)
	}
}

func TestNormalizeEntityForGraph(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "Terraform", "terraform"},
		{"tf alias", "tf", "terraform"},
		{"k8s alias", "k8s", "kubernetes"},
		{"pg alias", "pg", "postgresql"},
		{"postgres alias", "postgres", "postgresql"},
		{"no alias", "docker", "docker"},
		{"preserve original if no alias", "Claude", "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEntityForGraph(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeEntityForGraph(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
