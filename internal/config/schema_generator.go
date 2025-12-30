package config

import (
	"reflect"
	"strings"
)

// generateConfigSchema builds the configuration schema by introspecting
// Config and MinimalConfig structs. Tier classification is automatically
// derived: fields in MinimalConfig are "minimal", all others are "advanced".
func generateConfigSchema() *ConfigSchema {
	// Build map of field paths that exist in MinimalConfig
	minimalFields := buildMinimalFieldMap()

	// Generate items (sections and root fields) by walking Config struct
	items := generateConfigItems(minimalFields)

	// Hardcoded settings (already correct, no changes needed)
	hardcoded := getHardcodedSettings()

	return &ConfigSchema{
		Items:     items,
		Hardcoded: hardcoded,
	}
}

// buildMinimalFieldMap returns a map of field paths that exist in MinimalConfig.
// Field paths use dot notation: "daemon.log_level", "memory.root", etc.
// This map is used to automatically determine which fields are "minimal" tier.
func buildMinimalFieldMap() map[string]bool {
	minimalPaths := make(map[string]bool)

	// Walk MinimalConfig struct tree
	minimalType := reflect.TypeOf(MinimalConfig{})
	walkMinimalStructFields(minimalType, "", minimalPaths)

	return minimalPaths
}

// walkMinimalStructFields recursively walks MinimalConfig struct fields
// and records their paths in the provided map.
func walkMinimalStructFields(t reflect.Type, prefix string, paths map[string]bool) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name from yaml tag (matches mapstructure tag used in Config)
		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "omitempty" || tag == "-" {
			continue
		}

		// Remove ,omitempty suffix if present
		tag = strings.TrimSuffix(tag, ",omitempty")

		// Build field path
		fieldPath := tag
		if prefix != "" {
			fieldPath = prefix + "." + tag
		}

		// Mark this path as minimal
		paths[fieldPath] = true

		// If field is a struct, recursively walk its fields
		fieldType := field.Type
		if fieldType.Kind() == reflect.Struct {
			// For nested structs in MinimalConfig (e.g., MinimalClaudeConfig),
			// we need to get the section name from the parent field tag
			sectionName := tag
			if prefix == "" {
				// Top-level section (claude, daemon, mcp, etc.)
				walkMinimalStructFields(fieldType, sectionName, paths)
			}
		}
	}
}

// generateConfigItems walks the Config struct and generates SchemaItem
// for each top-level field, automatically deriving tier classifications.
// Returns a mix of RootField (for simple types) and SchemaSection (for structs).
func generateConfigItems(minimalFields map[string]bool) []SchemaItem {
	items := []SchemaItem{}

	configType := reflect.TypeOf(Config{})
	defaultConfig := reflect.ValueOf(DefaultConfig)

	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		fieldValue := defaultConfig.Field(i)

		// Get field name from mapstructure tag
		fieldTag := field.Tag.Get("mapstructure")
		if fieldTag == "" {
			continue
		}

		// Check if this is a simple type (root field) or nested struct (section)
		item := generateSchemaItem(fieldTag, field.Type, fieldValue, minimalFields)
		items = append(items, item)
	}

	return items
}

// generateSchemaItem generates a SchemaItem for a single config field.
// Returns RootField for simple types (string, int, bool, etc.) and
// SchemaSection for nested struct types, automatically deriving tier classifications.
func generateSchemaItem(name string, t reflect.Type, v reflect.Value, minimalFields map[string]bool) SchemaItem {
	// Get description
	description, ok := sectionDescriptions[name]
	if !ok {
		description = ""
	}

	// For simple types: create RootField
	switch t.Kind() {
	case reflect.String, reflect.Int, reflect.Int64, reflect.Bool, reflect.Float64:
		return RootField{
			Name:        name,
			Type:        getTypeString(t),
			Default:     v.Interface(),
			Tier:        determineTier(name, minimalFields),
			HotReload:   getHotReload(name),
			Description: getFieldDescription(name),
		}
	}

	// For struct types: create SchemaSection with fields
	fields := []SchemaField{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name from mapstructure tag
		fieldTag := field.Tag.Get("mapstructure")
		if fieldTag == "" {
			continue
		}

		// Build field path for lookups
		fieldPath := name + "." + fieldTag

		// Skip derived fields (computed from other config, not user-configurable)
		if isDerivedField(fieldPath) {
			continue
		}

		fields = append(fields, SchemaField{
			Name:        fieldTag,
			Type:        getTypeString(field.Type),
			Default:     fieldValue.Interface(),
			Tier:        determineTier(fieldPath, minimalFields),
			HotReload:   getHotReload(fieldPath),
			Description: getFieldDescription(fieldPath),
		})
	}

	return SchemaSection{
		Name:        name,
		Description: description,
		Fields:      fields,
	}
}

// determineTier automatically derives tier classification based on presence
// in MinimalConfig. This is the key innovation - tier is no longer manually
// maintained and cannot drift from reality.
func determineTier(fieldPath string, minimalFields map[string]bool) string {
	if minimalFields[fieldPath] {
		return "minimal"
	}
	return "advanced"
}

// isDerivedField returns true if the field is computed from other configuration
// and not directly user-configurable. Derived fields should not appear in schema.
func isDerivedField(fieldPath string) bool {
	derivedFields := map[string]bool{
		"semantic.enabled":   true, // Derived from semantic.api_key presence
		"embeddings.enabled": true, // Derived from embeddings.api_key presence
	}
	return derivedFields[fieldPath]
}

// getTypeString converts reflect.Type to schema type string
func getTypeString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int64:
		return "int"
	case reflect.Bool:
		return "bool"
	case reflect.Float64:
		return "float64"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String {
			return "[]string"
		}
		return "[]" + t.Elem().String()
	default:
		return t.String()
	}
}

// getHardcodedSettings returns hardcoded configuration constants.
func getHardcodedSettings() []HardcodedSetting {
	return []HardcodedSetting{
		{
			Name:   "ClaudeAPIKeyEnv",
			Value:  ClaudeAPIKeyEnv,
			Reason: "Standard Anthropic convention (used when semantic.provider=claude)",
		},
		{
			Name:   "OpenAIAPIKeyEnv",
			Value:  OpenAIAPIKeyEnv,
			Reason: "Standard OpenAI convention (used when semantic.provider=openai)",
		},
		{
			Name:   "GoogleAPIKeyEnv",
			Value:  GoogleAPIKeyEnv,
			Reason: "Standard Google convention (used when semantic.provider=gemini)",
		},
		{
			Name:   "EmbeddingsAPIKeyEnv",
			Value:  EmbeddingsAPIKeyEnv,
			Reason: "Standard OpenAI convention (for embeddings)",
		},
		{
			Name:   "GraphPasswordEnv",
			Value:  GraphPasswordEnv,
			Reason: "Standard FalkorDB convention",
		},
		{
			Name:   "AppDirName",
			Value:  AppDirName,
			Reason: "Application directory convention",
		},
		{
			Name:   "ConfigFile",
			Value:  ConfigFile,
			Reason: "Configuration file naming convention",
		},
		{
			Name:   "DaemonLogFile",
			Value:  DaemonLogFile,
			Reason: "Daemon log file naming convention",
		},
		{
			Name:   "DaemonPIDFile",
			Value:  DaemonPIDFile,
			Reason: "Daemon PID file naming convention",
		},
		{
			Name:   "MCPLogFile",
			Value:  MCPLogFile,
			Reason: "MCP log file naming convention",
		},
		{
			Name:   "EmbeddingsCacheEnabled",
			Value:  EmbeddingsCacheEnabled,
			Reason: "Always enabled for performance - no use case for disabling",
		},
		{
			Name:   "EmbeddingsBatchSize",
			Value:  EmbeddingsBatchSize,
			Reason: "Optimized for OpenAI API rate limits",
		},
		{
			Name:   "OutputShowRecentDays",
			Value:  OutputShowRecentDays,
			Reason: "Default recent files window (7 days)",
		},
	}
}

// getFieldDescription returns the description for a field path.
// Descriptions cannot be derived from reflection, so they're stored as metadata.
func getFieldDescription(fieldPath string) string {
	if desc, ok := fieldDescriptions[fieldPath]; ok {
		return desc
	}
	return ""
}

// getHotReload returns whether a field supports hot-reload.
// Hot-reload capability cannot be derived from reflection, so it's stored as metadata.
func getHotReload(fieldPath string) bool {
	if hotReload, ok := hotReloadSettings[fieldPath]; ok {
		return hotReload
	}
	return false
}

// Section descriptions (cannot be derived from reflection)
var sectionDescriptions = map[string]string{
	"memory":       "Memory directory configuration",
	"semantic":     "Semantic analysis provider configuration",
	"daemon":       "Background daemon configuration",
	"mcp":          "MCP server configuration",
	"graph":        "FalkorDB knowledge graph configuration",
	"embeddings":   "Embedding provider configuration",
	"integrations": "Integration framework configuration",
}

// Field descriptions (cannot be derived from reflection)
var fieldDescriptions = map[string]string{
	"memory.root":                          "Directory containing files to index (requires daemon restart)",
	"semantic.provider":                    "Semantic analysis provider (claude, openai, gemini)",
	"semantic.api_key":                     "Provider API key (or use provider-specific env var: ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY)",
	"semantic.model":                       "Model to use for semantic analysis (provider-specific)",
	"semantic.max_tokens":                  "Maximum tokens per API request (1-8192)",
	"semantic.timeout":                     "API request timeout in seconds (5-300)",
	"semantic.enable_vision":               "Enable vision API for image analysis",
	"semantic.max_file_size":               "Maximum file size in bytes for analysis (default: 10MB)",
	"semantic.cache_dir":                   "Directory for analysis cache (requires daemon restart)",
	"semantic.rate_limit_per_min":          "Maximum API calls per minute (provider-specific, 1-200)",
	"daemon.http_port":                     "HTTP API port (0 to disable)",
	"daemon.workers":                       "Number of concurrent worker threads (1-20)",
	"daemon.rate_limit_per_min":            "Maximum Claude API calls per minute (1-200)",
	"daemon.debounce_ms":                   "File change debounce delay in milliseconds (0-10000)",
	"daemon.full_rebuild_interval_minutes": "Minutes between full index rebuilds (0 to disable)",
	"daemon.log_file":                      "Log file path (requires daemon restart)",
	"daemon.log_level":                     "Log level (debug, info, warn, error)",
	"daemon.skip_hidden":                   "Skip hidden files and directories (starting with .)",
	"daemon.skip_dirs":                     "Directory names to skip during scanning",
	"daemon.skip_files":                    "Filenames to skip during scanning",
	"daemon.skip_extensions":               "File extensions to skip during scanning",
	"mcp.log_file":                         "MCP server log file path (requires MCP restart)",
	"mcp.log_level":                        "MCP server log level (requires MCP restart)",
	"mcp.daemon_host":                      "Daemon HTTP host for MCP server (requires MCP restart)",
	"mcp.daemon_port":                      "Daemon HTTP port for MCP server (requires MCP restart)",
	"graph.host":                           "FalkorDB host (requires daemon restart)",
	"graph.port":                           "FalkorDB port (default: 6379)",
	"graph.database":                       "Graph database name",
	"graph.password":                       "FalkorDB password (or use FALKORDB_PASSWORD env var)",
	"graph.similarity_threshold":           "Similarity threshold for related files (0.0-1.0)",
	"graph.max_similar_files":              "Maximum related files to return (1-100)",
	"embeddings.api_key":                   "OpenAI API key for embeddings (or use OPENAI_API_KEY env var)",
	"embeddings.provider":                  "Embedding provider (only 'openai' currently supported)",
	"embeddings.model":                     "Embedding model (text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002)",
	"embeddings.dimensions":                "Vector dimensions (must match model: 1536 for small/ada-002, 3072 for large)",
}

// Hot-reload settings (cannot be derived from reflection)
var hotReloadSettings = map[string]bool{
	"memory.root":                          false,
	"semantic.provider":                    true,
	"semantic.api_key":                     true,
	"semantic.model":                       true,
	"semantic.max_tokens":                  true,
	"semantic.timeout":                     true,
	"semantic.enable_vision":               true,
	"semantic.max_file_size":               true,
	"semantic.cache_dir":                   false,
	"semantic.rate_limit_per_min":          true,
	"daemon.http_port":                     true,
	"daemon.workers":                       true,
	"daemon.rate_limit_per_min":            true,
	"daemon.debounce_ms":                   true,
	"daemon.full_rebuild_interval_minutes": true,
	"daemon.log_file":                      false,
	"daemon.log_level":                     true,
	"daemon.skip_hidden":                   true,
	"daemon.skip_dirs":                     true,
	"daemon.skip_files":                    true,
	"daemon.skip_extensions":               true,
	"mcp.log_file":                         false,
	"mcp.log_level":                        false,
	"mcp.daemon_host":                      false,
	"mcp.daemon_port":                      false,
	"graph.host":                           false,
	"graph.port":                           false,
	"graph.database":                       false,
	"graph.password":                       false,
	"graph.similarity_threshold":           true,
	"graph.max_similar_files":              true,
	"embeddings.api_key":                   true,
	"embeddings.provider":                  true,
	"embeddings.model":                     true,
	"embeddings.dimensions":                true,
}
