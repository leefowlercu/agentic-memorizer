package config

import (
	"testing"
)

func TestGetConfigSchema(t *testing.T) {
	schema := GetConfigSchema()

	if schema == nil {
		t.Fatal("GetConfigSchema() returned nil")
	}

	// Test that all sections are present
	expectedSections := []string{
		"memory_root",
		"claude",
		"analysis",
		"daemon",
		"mcp",
		"graph",
		"embeddings",
		"integrations",
	}

	for _, expected := range expectedSections {
		found := false
		for _, section := range schema.Sections {
			if section.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected section %q not found in schema", expected)
		}
	}
}

func TestGetConfigSchema_AllFieldsHaveDescriptions(t *testing.T) {
	schema := GetConfigSchema()

	for _, section := range schema.Sections {
		for _, field := range section.Fields {
			if field.Description == "" {
				t.Errorf("field %s.%s has no description", section.Name, field.Name)
			}
		}
	}
}

func TestGetConfigSchema_AllFieldsHaveTypes(t *testing.T) {
	schema := GetConfigSchema()

	validTypes := map[string]bool{
		"string":   true,
		"int":      true,
		"int64":    true,
		"bool":     true,
		"float64":  true,
		"[]string": true,
	}

	for _, section := range schema.Sections {
		for _, field := range section.Fields {
			if field.Type == "" {
				t.Errorf("field %s.%s has no type", section.Name, field.Name)
			}
			if !validTypes[field.Type] {
				t.Errorf("field %s.%s has invalid type %q", section.Name, field.Name, field.Type)
			}
		}
	}
}

func TestGetConfigSchema_AllFieldsHaveTiers(t *testing.T) {
	schema := GetConfigSchema()

	validTiers := map[string]bool{
		"minimal":  true,
		"advanced": true,
	}

	for _, section := range schema.Sections {
		for _, field := range section.Fields {
			if field.Tier == "" {
				t.Errorf("field %s.%s has no tier", section.Name, field.Name)
			}
			if !validTiers[field.Tier] {
				t.Errorf("field %s.%s has invalid tier %q", section.Name, field.Name, field.Tier)
			}
		}
	}
}

func TestGetConfigSchema_HardcodedSettingsHaveReasons(t *testing.T) {
	schema := GetConfigSchema()

	if len(schema.Hardcoded) == 0 {
		t.Error("expected at least one hardcoded setting")
	}

	for _, hc := range schema.Hardcoded {
		if hc.Name == "" {
			t.Error("hardcoded setting has no name")
		}
		if hc.Reason == "" {
			t.Errorf("hardcoded setting %q has no reason", hc.Name)
		}
	}
}

func TestGetConfigSchema_ClaudeSectionHasNewFields(t *testing.T) {
	schema := GetConfigSchema()

	var claudeSection *SchemaSection
	for i := range schema.Sections {
		if schema.Sections[i].Name == "claude" {
			claudeSection = &schema.Sections[i]
			break
		}
	}

	if claudeSection == nil {
		t.Fatal("claude section not found")
	}

	// Check for new fields
	expectedFields := map[string]string{
		"timeout":       "advanced",
		"enable_vision": "advanced",
	}

	for fieldName, expectedTier := range expectedFields {
		found := false
		for _, field := range claudeSection.Fields {
			if field.Name == fieldName {
				found = true
				if field.Tier != expectedTier {
					t.Errorf("field %s has tier %q, expected %q", fieldName, field.Tier, expectedTier)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected field %q not found in claude section", fieldName)
		}
	}
}

func TestGetConfigSchema_EmbeddingsSectionHasNewFields(t *testing.T) {
	schema := GetConfigSchema()

	var embeddingsSection *SchemaSection
	for i := range schema.Sections {
		if schema.Sections[i].Name == "embeddings" {
			embeddingsSection = &schema.Sections[i]
			break
		}
	}

	if embeddingsSection == nil {
		t.Fatal("embeddings section not found")
	}

	// Check for new fields
	expectedFields := map[string]string{
		"provider":   "advanced",
		"model":      "advanced",
		"dimensions": "advanced",
	}

	for fieldName, expectedTier := range expectedFields {
		found := false
		for _, field := range embeddingsSection.Fields {
			if field.Name == fieldName {
				found = true
				if field.Tier != expectedTier {
					t.Errorf("field %s has tier %q, expected %q", fieldName, field.Tier, expectedTier)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected field %q not found in embeddings section", fieldName)
		}
	}
}
