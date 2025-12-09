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
	showSchemaFormat        string
	showSchemaAdvancedOnly  bool
	showSchemaHardcodedOnly bool
)

var ShowSchemaCmd = &cobra.Command{
	Use:   "show-schema",
	Short: "Show all configuration settings",
	Long: "\nDisplay complete configuration schema including minimal, advanced, and hardcoded settings.\n\n" +
		"This command shows all possible configuration options with their types, defaults, " +
		"and whether they appear in the initialized config file. Use this to discover " +
		"advanced settings not shown during initialization.",
	Example: `  # Show schema in text format
  agentic-memorizer config show-schema

  # Show schema in YAML format with examples
  agentic-memorizer config show-schema --format yaml

  # Show only advanced (hidden) settings
  agentic-memorizer config show-schema --advanced-only

  # Show only hardcoded (non-configurable) settings
  agentic-memorizer config show-schema --hardcoded-only`,
	PreRunE: validateShowSchema,
	RunE:    runShowSchema,
}

func init() {
	ShowSchemaCmd.Flags().StringVar(&showSchemaFormat, "format", "text",
		"Output format (text, yaml, json)")
	ShowSchemaCmd.Flags().BoolVar(&showSchemaAdvancedOnly, "advanced-only", false,
		"Show only advanced settings not in minimal config")
	ShowSchemaCmd.Flags().BoolVar(&showSchemaHardcodedOnly, "hardcoded-only", false,
		"Show only hardcoded (non-configurable) settings")
}

func validateShowSchema(cmd *cobra.Command, args []string) error {
	// Validate format
	validFormats := []string{"text", "yaml", "json"}
	found := slices.Contains(validFormats, showSchemaFormat)
	if !found {
		return fmt.Errorf("invalid format %q; valid formats are: %s", showSchemaFormat, strings.Join(validFormats, ", "))
	}

	// Validate mutually exclusive flags
	if showSchemaAdvancedOnly && showSchemaHardcodedOnly {
		return fmt.Errorf("cannot use both --advanced-only and --hardcoded-only")
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

	if !showSchemaHardcodedOnly {
		configurableSection := format.NewSection("CONFIGURABLE SETTINGS").SetLevel(1).AddDivider()

		for _, section := range schema.Sections {
			showSection := false
			if showSchemaAdvancedOnly {
				for _, field := range section.Fields {
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

			sectionGroup := format.NewSection(section.Name).SetLevel(2)

			for _, field := range section.Fields {
				if showSchemaAdvancedOnly && field.Tier != "advanced" {
					continue
				}

				hotReloadStr := "no"
				if field.HotReload {
					hotReloadStr = "yes"
				}

				fieldSection := format.NewSection(field.Name).SetLevel(3)
				fieldSection.AddKeyValue("Type", field.Type)
				fieldSection.AddKeyValue("Default", fmt.Sprintf("%v", formatDefault(field.Default)))
				fieldSection.AddKeyValue("Tier", field.Tier)
				fieldSection.AddKeyValue("Hot-Reload", hotReloadStr)
				fieldSection.AddKeyValue("Description", field.Description)

				sectionGroup.AddSubsection(fieldSection)
			}

			configurableSection.AddSubsection(sectionGroup)
		}

		mainSection.AddSubsection(configurableSection)
	}

	if !showSchemaAdvancedOnly {
		hardcodedSection := format.NewSection("HARDCODED SETTINGS (not configurable)").SetLevel(1).AddDivider()

		for _, hc := range schema.Hardcoded {
			hardcodedSection.AddKeyValue(hc.Name, fmt.Sprintf("%v (Reason: %s)", hc.Value, hc.Reason))
		}

		mainSection.AddSubsection(hardcodedSection)
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

	if !showSchemaHardcodedOnly {
		configurable := make(map[string]any)
		for _, section := range schema.Sections {
			sectionData := make(map[string]any)
			hasFields := false

			for _, field := range section.Fields {
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
				configurable[section.Name] = sectionData
			}
		}
		output["configurable"] = configurable
	}

	if !showSchemaAdvancedOnly {
		hardcoded := make(map[string]any)
		for _, hc := range schema.Hardcoded {
			hardcoded[hc.Name] = map[string]any{
				"value":  hc.Value,
				"reason": hc.Reason,
			}
		}
		output["hardcoded"] = hardcoded
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

	if !showSchemaHardcodedOnly {
		configurable := make(map[string]any)
		for _, section := range schema.Sections {
			sectionData := make(map[string]any)
			hasFields := false

			for _, field := range section.Fields {
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
				configurable[section.Name] = sectionData
			}
		}
		output["configurable"] = configurable
	}

	if !showSchemaAdvancedOnly {
		hardcoded := make(map[string]any)
		for _, hc := range schema.Hardcoded {
			hardcoded[hc.Name] = map[string]any{
				"value":  hc.Value,
				"reason": hc.Reason,
			}
		}
		output["hardcoded"] = hardcoded
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema to JSON; %w", err)
	}

	fmt.Println(string(data))
	return nil
}
