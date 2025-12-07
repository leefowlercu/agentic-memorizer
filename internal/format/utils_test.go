package format

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"single byte", 1, "1 B"},
		{"just under KB", 1023, "1023 B"},
		{"exactly 1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"exactly 1 MB", 1024 * 1024, "1.0 MB"},
		{"1.5 MB", 1536 * 1024, "1.5 MB"},
		{"exactly 1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"2.3 GB", 2469606195, "2.3 GB"},
		{"exactly 1 TB", 1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{"exactly 1 PB", 1024 * 1024 * 1024 * 1024 * 1024, "1.0 PB"},
		{"exactly 1 EB", 1024 * 1024 * 1024 * 1024 * 1024 * 1024, "1.0 EB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   int64
		expected string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"two digits", 42, "42"},
		{"three digits", 999, "999"},
		{"exactly 1000", 1000, "1,000"},
		{"1234", 1234, "1,234"},
		{"10000", 10000, "10,000"},
		{"100000", 100000, "100,000"},
		{"1000000", 1000000, "1,000,000"},
		{"1234567", 1234567, "1,234,567"},
		{"billion", 1234567890, "1,234,567,890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNumber(tt.number)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"100 milliseconds", 100 * time.Millisecond, "100ms"},
		{"500 milliseconds", 500 * time.Millisecond, "500ms"},
		{"1 second", 1 * time.Second, "1.0s"},
		{"5.5 seconds", 5500 * time.Millisecond, "5.5s"},
		{"1 minute", 1 * time.Minute, "1.0m"},
		{"2.5 minutes", 150 * time.Second, "2.5m"},
		{"1 hour", 1 * time.Hour, "1.0h"},
		{"3.5 hours", 210 * time.Minute, "3.5h"},
		{"1 day", 24 * time.Hour, "1.0d"},
		{"2.5 days", 60 * time.Hour, "2.5d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short maxLen", "hello", 3, "hel"},
		{"unicode string", "Hello, 世界!", 8, "Hello..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAlignText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		width     int
		alignment Alignment
		expected  string
	}{
		{"left align short", "hello", 10, AlignLeft, "hello     "},
		{"left align exact", "hello", 5, AlignLeft, "hello"},
		{"left align long", "hello world", 5, AlignLeft, "hello world"},
		{"right align short", "hello", 10, AlignRight, "     hello"},
		{"right align exact", "hello", 5, AlignRight, "hello"},
		{"center align even padding", "hi", 6, AlignCenter, "  hi  "},
		{"center align odd padding", "hi", 7, AlignCenter, "  hi   "},
		{"center align exact", "hello", 5, AlignCenter, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AlignText(tt.text, tt.width, tt.alignment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAlignText_WithANSI(t *testing.T) {
	// Text with ANSI codes should align correctly based on visible length
	redText := Red("hello")
	result := AlignText(redText, 10, AlignLeft)

	// Should have 5 spaces added (10 - 5 visible chars)
	plainResult := StripANSI(result)
	assert.Equal(t, "hello     ", plainResult)
	assert.Contains(t, result, colorRed) // Should preserve color codes
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "hello", "hello"},
		{"red text", Red("hello"), "hello"},
		{"green text", Green("world"), "world"},
		{"bold text", Bold("test"), "test"},
		{"multiple codes", Red(Bold("hello")) + " " + Green("world"), "hello world"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripANSI(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStatusSymbol(t *testing.T) {
	tests := []struct {
		severity StatusSeverity
		expected string
	}{
		{StatusSuccess, SymbolSuccess},
		{StatusInfo, SymbolInfo},
		{StatusWarning, SymbolWarning},
		{StatusError, SymbolError},
		{StatusRunning, SymbolRunning},
		{StatusStopped, SymbolStopped},
		{"unknown", SymbolInfo}, // Default for unknown
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			result := GetStatusSymbol(tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function func(string) string
		text     string
		contains string
	}{
		{"Green", Green, "test", colorGreen},
		{"Red", Red, "test", colorRed},
		{"Yellow", Yellow, "test", colorYellow},
		{"Blue", Blue, "test", colorBlue},
		{"Bold", Bold, "test", colorBold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function(tt.text)
			assert.Contains(t, result, tt.contains)
			assert.Contains(t, result, tt.text)
			assert.Contains(t, result, colorReset)
		})
	}
}

func BenchmarkFormatBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatBytes(1234567890)
	}
}

func BenchmarkFormatNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatNumber(1234567890)
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	duration := 2*time.Hour + 30*time.Minute
	for i := 0; i < b.N; i++ {
		FormatDuration(duration)
	}
}

func BenchmarkStripANSI(b *testing.B) {
	text := Red(Bold("hello")) + " " + Green("world")
	for i := 0; i < b.N; i++ {
		StripANSI(text)
	}
}
