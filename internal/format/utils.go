package format

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Status symbols
const (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "⚠"
	SymbolInfo    = "○"
	SymbolRunning = "▸"
	SymbolStopped = "■"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorBold   = "\033[1m"
)

// FormatBytes formats bytes into human-readable format (B, KB, MB, GB, TB, PB, EB)
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatNumber formats a number with thousands separators
func FormatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// FormatDuration formats a duration into human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	days := d.Hours() / 24
	return fmt.Sprintf("%.1fd", days)
}

// TruncateString truncates a string to maxLen characters, adding "..." if truncated
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// AlignText aligns text within a given width
func AlignText(text string, width int, align Alignment) string {
	// Strip ANSI codes for length calculation
	plainText := StripANSI(text)
	textLen := len(plainText)

	if textLen >= width {
		return text
	}

	padding := width - textLen

	switch align {
	case AlignLeft:
		return text + strings.Repeat(" ", padding)
	case AlignRight:
		return strings.Repeat(" ", padding) + text
	case AlignCenter:
		leftPad := padding / 2
		rightPad := padding - leftPad
		return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
	default:
		return text
	}
}

// StripANSI removes ANSI escape codes from a string
func StripANSI(s string) string {
	// Regex to match ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiRegex.ReplaceAllString(s, "")
}

// GetStatusSymbol returns the appropriate symbol for a status severity
func GetStatusSymbol(severity StatusSeverity) string {
	symbols := map[StatusSeverity]string{
		StatusSuccess: SymbolSuccess,
		StatusInfo:    SymbolInfo,
		StatusWarning: SymbolWarning,
		StatusError:   SymbolError,
		StatusRunning: SymbolRunning,
		StatusStopped: SymbolStopped,
	}

	if symbol, ok := symbols[severity]; ok {
		return symbol
	}
	return SymbolInfo // Default
}

// Color helper functions

// Green wraps text in green color codes
func Green(text string) string {
	return colorGreen + text + colorReset
}

// Red wraps text in red color codes
func Red(text string) string {
	return colorRed + text + colorReset
}

// Yellow wraps text in yellow color codes
func Yellow(text string) string {
	return colorYellow + text + colorReset
}

// Blue wraps text in blue color codes
func Blue(text string) string {
	return colorBlue + text + colorReset
}

// Bold wraps text in bold formatting codes
func Bold(text string) string {
	return colorBold + text + colorReset
}
