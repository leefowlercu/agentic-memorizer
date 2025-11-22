package version

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		gitCommit      string
		buildDate      string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:      "default values",
			version:   "dev",
			gitCommit: "unknown",
			buildDate: "unknown",
			wantContains: []string{
				"dev",
				"commit: unknown",
				"built: unknown",
			},
		},
		{
			name:      "production build",
			version:   "v1.0.0",
			gitCommit: "abc123def456",
			buildDate: "2024-01-15T10:30:00Z",
			wantContains: []string{
				"v1.0.0",
				"commit: abc123def456",
				"built: 2024-01-15T10:30:00Z",
			},
		},
		{
			name:      "semantic version",
			version:   "v2.3.4",
			gitCommit: "fedcba987654",
			buildDate: "2024-12-25T00:00:00Z",
			wantContains: []string{
				"v2.3.4",
				"commit: fedcba987654",
				"built: 2024-12-25T00:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origVersion := Version
			origCommit := GitCommit
			origDate := BuildDate

			// Set test values
			Version = tt.version
			GitCommit = tt.gitCommit
			BuildDate = tt.buildDate

			// Restore after test
			defer func() {
				Version = origVersion
				GitCommit = origCommit
				BuildDate = origDate
			}()

			got := GetVersion()

			// Check all required substrings are present
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("GetVersion() = %q, want to contain %q", got, want)
				}
			}

			// Check unwanted substrings are not present
			for _, unwanted := range tt.wantNotContain {
				if strings.Contains(got, unwanted) {
					t.Errorf("GetVersion() = %q, should not contain %q", got, unwanted)
				}
			}
		})
	}
}

func TestGetVersion_Format(t *testing.T) {
	// Save originals
	origVersion := Version
	origCommit := GitCommit
	origDate := BuildDate
	defer func() {
		Version = origVersion
		GitCommit = origCommit
		BuildDate = origDate
	}()

	Version = "v1.0.0"
	GitCommit = "abc123"
	BuildDate = "2024-01-01"

	got := GetVersion()
	expected := "v1.0.0 (commit: abc123, built: 2024-01-01)"

	if got != expected {
		t.Errorf("GetVersion() = %q, want %q", got, expected)
	}
}

func TestGetShortVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "dev version",
			version: "dev",
			want:    "dev",
		},
		{
			name:    "semantic version",
			version: "v1.2.3",
			want:    "v1.2.3",
		},
		{
			name:    "version with prerelease",
			version: "v2.0.0-beta.1",
			want:    "v2.0.0-beta.1",
		},
		{
			name:    "empty version falls back to embedded",
			version: "",
			want:    "0.10.0", // Falls back to embedded VERSION file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original
			orig := Version
			defer func() { Version = orig }()

			Version = tt.version
			got := GetShortVersion()

			if got != tt.want {
				t.Errorf("GetShortVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetGitCommit(t *testing.T) {
	tests := []struct {
		name      string
		gitCommit string
		want      string
	}{
		{
			name:      "unknown commit",
			gitCommit: "unknown",
			want:      "unknown",
		},
		{
			name:      "short commit hash",
			gitCommit: "abc123",
			want:      "abc123",
		},
		{
			name:      "full commit hash",
			gitCommit: "abc123def456789012345678901234567890abcd",
			want:      "abc123def456789012345678901234567890abcd",
		},
		{
			name:      "empty commit",
			gitCommit: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original
			orig := GitCommit
			defer func() { GitCommit = orig }()

			GitCommit = tt.gitCommit
			got := GetGitCommit()

			if got != tt.want {
				t.Errorf("GetGitCommit() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBuildDate(t *testing.T) {
	tests := []struct {
		name      string
		buildDate string
		want      string
	}{
		{
			name:      "unknown date",
			buildDate: "unknown",
			want:      "unknown",
		},
		{
			name:      "ISO 8601 format",
			buildDate: "2024-01-15T10:30:00Z",
			want:      "2024-01-15T10:30:00Z",
		},
		{
			name:      "RFC 3339 format",
			buildDate: "2024-12-25T14:30:00+00:00",
			want:      "2024-12-25T14:30:00+00:00",
		},
		{
			name:      "empty date",
			buildDate: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original
			orig := BuildDate
			defer func() { BuildDate = orig }()

			BuildDate = tt.buildDate
			got := GetBuildDate()

			if got != tt.want {
				t.Errorf("GetBuildDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionGlobals(t *testing.T) {
	// Test that globals can be modified (important for build-time injection)
	origVersion := Version
	origCommit := GitCommit
	origDate := BuildDate

	defer func() {
		Version = origVersion
		GitCommit = origCommit
		BuildDate = origDate
	}()

	// Modify globals
	Version = "test-version"
	GitCommit = "test-commit"
	BuildDate = "test-date"

	// Verify they were changed
	if Version != "test-version" {
		t.Errorf("Version = %q, want %q", Version, "test-version")
	}
	if GitCommit != "test-commit" {
		t.Errorf("GitCommit = %q, want %q", GitCommit, "test-commit")
	}
	if BuildDate != "test-date" {
		t.Errorf("BuildDate = %q, want %q", BuildDate, "test-date")
	}
}

func TestVersionIntegration(t *testing.T) {
	// Save originals
	origVersion := Version
	origCommit := GitCommit
	origDate := BuildDate
	defer func() {
		Version = origVersion
		GitCommit = origCommit
		BuildDate = origDate
	}()

	// Set realistic values
	Version = "v0.9.0"
	GitCommit = "d147fda"
	BuildDate = "2025-11-20T20:00:00Z"

	// Test all getters work together
	fullVersion := GetVersion()
	shortVersion := GetShortVersion()
	commit := GetGitCommit()
	date := GetBuildDate()

	if shortVersion != "v0.9.0" {
		t.Errorf("GetShortVersion() = %q, want %q", shortVersion, "v0.9.0")
	}

	if commit != "d147fda" {
		t.Errorf("GetGitCommit() = %q, want %q", commit, "d147fda")
	}

	if date != "2025-11-20T20:00:00Z" {
		t.Errorf("GetBuildDate() = %q, want %q", date, "2025-11-20T20:00:00Z")
	}

	expectedFull := "v0.9.0 (commit: d147fda, built: 2025-11-20T20:00:00Z)"
	if fullVersion != expectedFull {
		t.Errorf("GetVersion() = %q, want %q", fullVersion, expectedFull)
	}
}
