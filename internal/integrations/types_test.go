package integrations

import "testing"

func TestOutputFormat_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		format OutputFormat
		want   bool
	}{
		{
			name:   "FormatXML is valid",
			format: FormatXML,
			want:   true,
		},
		{
			name:   "FormatJSON is valid",
			format: FormatJSON,
			want:   true,
		},
		{
			name:   "invalid format",
			format: OutputFormat("yaml"),
			want:   false,
		},
		{
			name:   "empty format",
			format: OutputFormat(""),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.format.IsValid()
			if got != tt.want {
				t.Errorf("OutputFormat.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		name   string
		format OutputFormat
		want   string
	}{
		{
			name:   "FormatXML",
			format: FormatXML,
			want:   "xml",
		},
		{
			name:   "FormatJSON",
			format: FormatJSON,
			want:   "json",
		},
		{
			name:   "custom format",
			format: OutputFormat("custom"),
			want:   "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.format.String()
			if got != tt.want {
				t.Errorf("OutputFormat.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
