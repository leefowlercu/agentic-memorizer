// Package format provides centralized CLI output formatting for agentic-memorizer.
//
// The package implements a three-tier architecture:
//
//  1. Builders - Construct structured output (Section, Table, List, Progress, Status, Error)
//  2. Formatters - Render builders to specific formats (Text, JSON, YAML, Markdown, XML)
//  3. Writers - Handle buffered I/O with error handling
//
// # Basic Usage
//
// Create a section with key-value pairs:
//
//	section := format.NewSection("Daemon Status")
//	section.AddKeyValue("Status", "Running")
//	section.AddKeyValue("PID", "12345")
//
//	formatter := format.GetFormatter("text")
//	output, _ := formatter.Format(section)
//	fmt.Print(output)
//
// # Multi-Format Support
//
// Commands can support multiple output formats:
//
//	cmd.Flags().String("format", "text", "Output format (text|json|yaml)")
//
//	formatStr, _ := cmd.Flags().GetString("format")
//	formatter, _ := format.GetFormatter(formatStr)
//	output, _ := formatter.Format(section)
//	fmt.Print(output)
//
// # Builder Types
//
//   - Section: Hierarchical key-value pairs with headers and subsections
//   - Table: Columnar data with headers, alignment, and compact mode
//   - List: Ordered/unordered lists with nesting support
//   - Progress: Progress bars, spinners, and percentage indicators
//   - Status: Status messages with severity levels and symbols
//   - Error: Structured error formatting with suggestions
//
// # Shared Utilities
//
// The package provides common formatting utilities:
//
//   - FormatBytes(int64) - Human-readable byte sizes (1.5 MB, 2.3 GB)
//   - FormatNumber(int64) - Numbers with thousands separators (1,234,567)
//   - FormatDuration(time.Duration) - Human-readable durations
//   - GetStatusSymbol(StatusSeverity) - Consistent status symbols (✓, ✗, ○, ⚠)
//
// # Thread Safety
//
// The formatter registry is thread-safe and can be accessed concurrently.
// Individual builders and formatters are not thread-safe and should be used
// from a single goroutine.
//
// # Testing
//
// When testing commands with formatted output:
//
//  1. Use golden files for complex output verification
//  2. Test each supported format separately
//  3. Verify E2E tests pass after refactoring
//
// See internal/format/README.md for comprehensive documentation.
package format
