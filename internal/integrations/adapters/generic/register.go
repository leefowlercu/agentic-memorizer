package generic

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers generic adapters with the global registry
// These provide fallback support for frameworks without specific adapters
func init() {
	// Register generic adapters for common frameworks that don't have specific support yet
	integrations.GlobalRegistry().Register(
		NewGenericAdapter(
			"continue",
			"Continue.dev integration (manual setup required)",
			integrations.FormatMarkdown,
		),
	)

	integrations.GlobalRegistry().Register(
		NewGenericAdapter(
			"cline",
			"Cline integration (manual setup required)",
			integrations.FormatMarkdown,
		),
	)

	integrations.GlobalRegistry().Register(
		NewGenericAdapter(
			"aider",
			"Aider integration (manual setup required)",
			integrations.FormatMarkdown,
		),
	)

	integrations.GlobalRegistry().Register(
		NewGenericAdapter(
			"cursor",
			"Cursor AI integration (manual setup required)",
			integrations.FormatMarkdown,
		),
	)

	integrations.GlobalRegistry().Register(
		NewGenericAdapter(
			"custom",
			"Custom integration (manual setup required)",
			integrations.FormatXML,
		),
	)
}
