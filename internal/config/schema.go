package config

// ConfigSchema describes all configuration settings including
// configurable settings (minimal and advanced) and hardcoded conventions.
type ConfigSchema struct {
	Sections  []SchemaSection
	Hardcoded []HardcodedSetting
}

// SchemaSection represents a config section (claude, daemon, etc.)
type SchemaSection struct {
	Name        string
	Description string
	Fields      []SchemaField
}

// SchemaField describes a single configuration field
type SchemaField struct {
	Name        string
	Type        string // "string", "int", "bool", "float64", "[]string"
	Default     any
	Tier        string // "minimal" or "advanced"
	HotReload   bool   // true if hot-reloadable without daemon restart
	Description string
}

// HardcodedSetting describes a non-configurable constant
type HardcodedSetting struct {
	Name   string
	Value  any
	Reason string
}

// GetConfigSchema returns the complete configuration schema.
// Schema is generated automatically via reflection on Config and MinimalConfig structs.
// Tier classification is derived: fields in MinimalConfig are "minimal", others are "advanced".
func GetConfigSchema() *ConfigSchema {
	return generateConfigSchema()
}
