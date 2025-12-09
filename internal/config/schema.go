package config

// SchemaItem represents an item in the configuration schema.
// It can be either a SchemaSection (for nested struct fields) or a
// RootField (for simple type fields at the root level).
type SchemaItem interface {
	isSchemaItem() // Marker method to restrict implementation
}

// ConfigSchema describes all configuration settings including
// configurable settings (minimal and advanced) and hardcoded conventions.
type ConfigSchema struct {
	Items     []SchemaItem // Can contain both RootField and SchemaSection
	Hardcoded []HardcodedSetting
}

// RootField represents a simple-type field at the root of the configuration.
// Unlike nested sections, root fields are displayed directly without a wrapping section.
type RootField struct {
	Name        string
	Type        string
	Default     any
	Tier        string
	HotReload   bool
	Description string
}

// isSchemaItem marks RootField as implementing SchemaItem interface
func (RootField) isSchemaItem() {}

// SchemaSection represents a config section (claude, daemon, etc.)
type SchemaSection struct {
	Name        string
	Description string
	Fields      []SchemaField
}

// isSchemaItem marks SchemaSection as implementing SchemaItem interface
func (SchemaSection) isSchemaItem() {}

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
