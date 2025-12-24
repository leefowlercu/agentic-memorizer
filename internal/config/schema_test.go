package config

import (
	"testing"
)

func TestGetConfigSchema(t *testing.T) {
	schema := GetConfigSchema()

	if schema == nil {
		t.Fatal("GetConfigSchema() returned nil")
	}

	// Test that all items are present (both RootFields and Sections)
	expectedItems := []string{
		"memory_root",
		"claude",
		"analysis",
		"daemon",
		"mcp",
		"graph",
		"embeddings",
	}

	for _, expected := range expectedItems {
		found := false
		for _, item := range schema.Items {
			var name string
			switch v := item.(type) {
			case RootField:
				name = v.Name
			case SchemaSection:
				name = v.Name
			}
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected item %q not found in schema", expected)
		}
	}
}

func TestGetConfigSchema_AllFieldsHaveDescriptions(t *testing.T) {
	schema := GetConfigSchema()

	for _, item := range schema.Items {
		switch v := item.(type) {
		case RootField:
			if v.Description == "" {
				t.Errorf("root field %s has no description", v.Name)
			}
		case SchemaSection:
			for _, field := range v.Fields {
				if field.Description == "" {
					t.Errorf("field %s.%s has no description", v.Name, field.Name)
				}
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

	for _, item := range schema.Items {
		switch v := item.(type) {
		case RootField:
			if v.Type == "" {
				t.Errorf("root field %s has no type", v.Name)
			}
			if !validTypes[v.Type] {
				t.Errorf("root field %s has invalid type %q", v.Name, v.Type)
			}
		case SchemaSection:
			for _, field := range v.Fields {
				if field.Type == "" {
					t.Errorf("field %s.%s has no type", v.Name, field.Name)
				}
				if !validTypes[field.Type] {
					t.Errorf("field %s.%s has invalid type %q", v.Name, field.Name, field.Type)
				}
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

	for _, item := range schema.Items {
		switch v := item.(type) {
		case RootField:
			if v.Tier == "" {
				t.Errorf("root field %s has no tier", v.Name)
			}
			if !validTiers[v.Tier] {
				t.Errorf("root field %s has invalid tier %q", v.Name, v.Tier)
			}
		case SchemaSection:
			for _, field := range v.Fields {
				if field.Tier == "" {
					t.Errorf("field %s.%s has no tier", v.Name, field.Name)
				}
				if !validTiers[field.Tier] {
					t.Errorf("field %s.%s has invalid tier %q", v.Name, field.Name, field.Tier)
				}
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
	for _, item := range schema.Items {
		if section, ok := item.(SchemaSection); ok && section.Name == "claude" {
			claudeSection = &section
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
	for _, item := range schema.Items {
		if section, ok := item.(SchemaSection); ok && section.Name == "embeddings" {
			embeddingsSection = &section
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
