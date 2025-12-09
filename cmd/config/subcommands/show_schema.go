package subcommands

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	showSchemaFormat       string
	showSchemaAdvancedOnly bool
)

var ShowSchemaCmd = &cobra.Command{
	Use:   "show-schema",
	Short: "Show all configuration settings",
	Long: "\nDisplay complete configuration schema including minimal and advanced settings.\n\n" +
		"This command shows all possible configuration options with their types, defaults, " +
		"and whether they appear in the initialized config file. Use this to discover " +
		"advanced settings not shown during initialization.",
	Example: `  # Show schema in text format
  agentic-memorizer config show-schema

  # Show schema in YAML format with examples
  agentic-memorizer config show-schema --format yaml

  # Show only advanced (hidden) settings
  agentic-memorizer config show-schema --advanced-only`,
	PreRunE: validateShowSchema,
	RunE:    runShowSchema,
}

func init() {
	ShowSchemaCmd.Flags().StringVar(&showSchemaFormat, "format", "text",
		"Output format (text, yaml, json)")
	ShowSchemaCmd.Flags().BoolVar(&showSchemaAdvancedOnly, "advanced-only", false,
		"Show only advanced settings not in minimal config")
}

func validateShowSchema(cmd *cobra.Command, args []string) error {
	// Validate format
	validFormats := []string{"text", "yaml", "json"}
	found := slices.Contains(validFormats, showSchemaFormat)
	if !found {
		return fmt.Errorf("invalid format %q; valid formats are: %s", showSchemaFormat, strings.Join(validFormats, ", "))
	}

	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runShowSchema(cmd *cobra.Command, args []string) error {
	schema := config.GetConfigSchema()

	switch showSchemaFormat {
	case "text":
		return printSchemaText(schema)
	case "yaml":
		return printSchemaYAML(schema)
	case "json":
		return printSchemaJSON(schema)
	default:
		return fmt.Errorf("invalid format: %s", showSchemaFormat)
	}
}

func printSchemaText(schema *config.ConfigSchema) error {
	// Build main section
	mainSection := format.NewSection("Configuration Schema").AddDivider()

	for _, item := range schema.Items {
		switch v := item.(type) {
		case config.RootField:
			// Handle root field directly
			if showSchemaAdvancedOnly && v.Tier != "advanced" {
				continue
			}

			fieldSection := format.NewSection(v.Name).SetLevel(0)
			hotReloadStr := "no"
			if v.HotReload {
				hotReloadStr = "yes"
			}
			fieldSection.AddKeyValue("Type", v.Type)
			fieldSection.AddKeyValue("Default", fmt.Sprintf("%v", formatDefault(v.Default)))
			fieldSection.AddKeyValue("Tier", v.Tier)
			fieldSection.AddKeyValue("Hot-Reload", hotReloadStr)
			fieldSection.AddKeyValue("Description", v.Description)

			mainSection.AddSubsection(fieldSection)

		case config.SchemaSection:
			// Handle section (existing logic)
			showSection := false
			if showSchemaAdvancedOnly {
				for _, field := range v.Fields {
					if field.Tier == "advanced" {
						showSection = true
						break
					}
				}
			} else {
				showSection = true
			}

			if !showSection {
				continue
			}

			sectionGroup := format.NewSection(v.Name).SetLevel(0)

			for _, field := range v.Fields {
				if showSchemaAdvancedOnly && field.Tier != "advanced" {
					continue
				}

				hotReloadStr := "no"
				if field.HotReload {
					hotReloadStr = "yes"
				}

				fieldSection := format.NewSection(field.Name).SetLevel(1)
				fieldSection.AddKeyValue("Type", field.Type)
				fieldSection.AddKeyValue("Default", fmt.Sprintf("%v", formatDefault(field.Default)))
				fieldSection.AddKeyValue("Tier", field.Tier)
				fieldSection.AddKeyValue("Hot-Reload", hotReloadStr)
				fieldSection.AddKeyValue("Description", field.Description)

				sectionGroup.AddSubsection(fieldSection)
			}

			mainSection.AddSubsection(sectionGroup)
		}
	}

	// Format and write output
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(mainSection)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}
	fmt.Println(output)

	return nil
}

// formatDefault formats a default value for display
func formatDefault(v any) string {
	switch val := v.(type) {
	case []string:
		if len(val) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[%s]", strings.Join(val, ", "))
	case string:
		if val == "" {
			return "(empty)"
		}
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

func printSchemaYAML(schema *config.ConfigSchema) error {
	output := make(map[string]any)

	for _, item := range schema.Items {
		switch v := item.(type) {
		case config.RootField:
			if showSchemaAdvancedOnly && v.Tier != "advanced" {
				continue
			}

			fieldData := map[string]any{
				"type":        v.Type,
				"default":     v.Default,
				"tier":        v.Tier,
				"hot_reload":  v.HotReload,
				"description": v.Description,
			}
			output[v.Name] = fieldData

		case config.SchemaSection:
			sectionData := make(map[string]any)
			hasFields := false

			for _, field := range v.Fields {
				if showSchemaAdvancedOnly && field.Tier != "advanced" {
					continue
				}
				hasFields = true

				fieldData := map[string]any{
					"type":        field.Type,
					"default":     field.Default,
					"tier":        field.Tier,
					"hot_reload":  field.HotReload,
					"description": field.Description,
				}
				sectionData[field.Name] = fieldData
			}

			if hasFields {
				output[v.Name] = sectionData
			}
		}
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal schema to YAML; %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func printSchemaJSON(schema *config.ConfigSchema) error {
	output := make(map[string]any)

	for _, item := range schema.Items {
		switch v := item.(type) {
		case config.RootField:
			if showSchemaAdvancedOnly && v.Tier != "advanced" {
				continue
			}

			fieldData := map[string]any{
				"type":        v.Type,
				"default":     v.Default,
				"tier":        v.Tier,
				"hot_reload":  v.HotReload,
				"description": v.Description,
			}
			output[v.Name] = fieldData

		case config.SchemaSection:
			sectionData := make(map[string]any)
			hasFields := false

			for _, field := range v.Fields {
				if showSchemaAdvancedOnly && field.Tier != "advanced" {
					continue
				}
				hasFields = true

				fieldData := map[string]any{
					"type":        field.Type,
					"default":     field.Default,
					"tier":        field.Tier,
					"hot_reload":  field.HotReload,
					"description": field.Description,
				}
				sectionData[field.Name] = fieldData
			}

			if hasFields {
				output[v.Name] = sectionData
			}
		}
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema to JSON; %w", err)
	}

	fmt.Println(string(data))
	return nil
}
