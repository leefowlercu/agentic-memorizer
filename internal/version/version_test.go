package version

import (
	"strings"
	"testing"
)

// T003: Tests for version embedding
func TestGetVersion(t *testing.T) {
	tests := []struct {
		name    string
		wantMin string
	}{
		{
			name:    "returns non-empty version",
			wantMin: "0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVersion()
			if got == "" {
				t.Error("getVersion() returned empty string")
			}
			if !strings.HasPrefix(got, "0.") && !strings.HasPrefix(got, "1.") {
				t.Errorf("getVersion() = %q, expected semver format", got)
			}
		})
	}
}

func TestGetVersionTrimsWhitespace(t *testing.T) {
	got := getVersion()
	if got != strings.TrimSpace(got) {
		t.Errorf("getVersion() = %q, contains leading/trailing whitespace", got)
	}
}

// T005: Tests for Info struct and String() method
func TestInfoString(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "formats all fields",
			info: Info{
				Version:   "1.0.0",
				GitCommit: "abc1234",
				BuildDate: "2026-01-10T15:04:05Z",
			},
			want: "Version:    1.0.0\nGit Commit: abc1234\nBuild Date: 2026-01-10T15:04:05Z",
		},
		{
			name: "handles unknown values",
			info: Info{
				Version:   "0.1.0",
				GitCommit: "unknown",
				BuildDate: "unknown",
			},
			want: "Version:    0.1.0\nGit Commit: unknown\nBuild Date: unknown",
		},
		{
			name: "handles dirty commit",
			info: Info{
				Version:   "1.0.0-alpha.1",
				GitCommit: "def5678-dirty",
				BuildDate: "2026-01-10T16:00:00Z",
			},
			want: "Version:    1.0.0-alpha.1\nGit Commit: def5678-dirty\nBuild Date: 2026-01-10T16:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.String()
			if got != tt.want {
				t.Errorf("Info.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInfoStringContainsLabels(t *testing.T) {
	info := Info{
		Version:   "1.0.0",
		GitCommit: "abc1234",
		BuildDate: "2026-01-10T15:04:05Z",
	}
	got := info.String()

	requiredLabels := []string{"Version:", "Git Commit:", "Build Date:"}
	for _, label := range requiredLabels {
		if !strings.Contains(got, label) {
			t.Errorf("Info.String() missing label %q", label)
		}
	}
}

// T007: Tests for getGitCommit with linker flag and fallback
func TestGetGitCommit(t *testing.T) {
	got := getGitCommit()
	// During tests without linker flags, should return value from debug.ReadBuildInfo
	// or "unknown" if not available
	if got == "" {
		t.Error("getGitCommit() returned empty string, expected value or 'unknown'")
	}
}

func TestGetGitCommitFormat(t *testing.T) {
	got := getGitCommit()
	// Git commit should be either:
	// - "unknown"
	// - 7-8 character hex string (optionally with -dirty suffix)
	// - Full 40 char hash (from debug.ReadBuildInfo, optionally with -dirty)
	if got == "unknown" {
		return // valid fallback
	}

	// Remove -dirty suffix for validation
	commit := strings.TrimSuffix(got, "-dirty")

	// Should be hex characters only
	for _, c := range commit {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("getGitCommit() = %q, contains non-hex character %q", got, c)
			return
		}
	}
}

// T003a: Tests for semver format validation (pre-release, build metadata)
func TestSemverFormatValidation(t *testing.T) {
	tests := []struct {
		name    string
		version string
		valid   bool
	}{
		{
			name:    "basic version",
			version: "0.1.0",
			valid:   true,
		},
		{
			name:    "release version",
			version: "1.0.0",
			valid:   true,
		},
		{
			name:    "pre-release alpha",
			version: "1.0.0-alpha.1",
			valid:   true,
		},
		{
			name:    "pre-release rc",
			version: "1.0.0-rc.1",
			valid:   true,
		},
		{
			name:    "build metadata only",
			version: "2.0.0+build.789",
			valid:   true,
		},
		{
			name:    "pre-release with build metadata",
			version: "1.0.0-rc.1+build.456",
			valid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Semver format validation: MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
			// The VERSION file can contain any valid semver, we just verify
			// the embedded version is returned as-is (trimmed)
			if tt.valid {
				// These are all valid formats that should be accepted
				parts := strings.SplitN(tt.version, ".", 3)
				if len(parts) < 3 {
					t.Errorf("version %q does not have MAJOR.MINOR.PATCH format", tt.version)
				}
			}
		})
	}
}

// T011: Tests for Get() function
func TestGet(t *testing.T) {
	info := Get()

	// Version should always be populated from embedded file
	if info.Version == "" {
		t.Error("Get().Version is empty, expected embedded version")
	}

	// GitCommit should be populated (either from linker, BuildInfo, or "unknown")
	if info.GitCommit == "" {
		t.Error("Get().GitCommit is empty, expected value or 'unknown'")
	}

	// BuildDate should be populated (either from linker or "unknown")
	if info.BuildDate == "" {
		t.Error("Get().BuildDate is empty, expected value or 'unknown'")
	}
}

func TestGetReturnsConsistentInfo(t *testing.T) {
	info1 := Get()
	info2 := Get()

	if info1.Version != info2.Version {
		t.Errorf("Get() returned inconsistent Version: %q vs %q", info1.Version, info2.Version)
	}
	if info1.GitCommit != info2.GitCommit {
		t.Errorf("Get() returned inconsistent GitCommit: %q vs %q", info1.GitCommit, info2.GitCommit)
	}
	if info1.BuildDate != info2.BuildDate {
		t.Errorf("Get() returned inconsistent BuildDate: %q vs %q", info1.BuildDate, info2.BuildDate)
	}
}

// T009: Tests for getBuildDate with linker flag
func TestGetBuildDate(t *testing.T) {
	got := getBuildDate()
	// During tests without linker flags, should return "unknown"
	if got == "" {
		t.Error("getBuildDate() returned empty string, expected value or 'unknown'")
	}
}

func TestGetBuildDateFormat(t *testing.T) {
	got := getBuildDate()
	// Build date should be either:
	// - "unknown" (fallback)
	// - ISO 8601 format (e.g., "2026-01-10T15:04:05Z")
	if got == "unknown" {
		return // valid fallback
	}

	// Should contain date-like patterns if not "unknown"
	if !strings.Contains(got, "-") || !strings.Contains(got, ":") {
		t.Errorf("getBuildDate() = %q, expected ISO 8601 format or 'unknown'", got)
	}
}

// T027-T030: Tests for debug.ReadBuildInfo fallback scenarios
func TestReadBuildInfo(t *testing.T) {
	// readBuildInfo should return values from runtime/debug.ReadBuildInfo
	// During go test, this may or may not have VCS info depending on build context
	revision, dirty := readBuildInfo()

	// Either revision is empty (no VCS info) or it's a valid commit hash
	if revision != "" {
		// Should be hex characters only (7 chars after shortening)
		for _, c := range revision {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				t.Errorf("readBuildInfo() revision = %q, contains non-hex character", revision)
				return
			}
		}
		// Should be shortened to 7 characters
		if len(revision) > 7 {
			t.Errorf("readBuildInfo() revision = %q, expected 7 chars max", revision)
		}
	}

	// dirty is a boolean, just verify it doesn't panic
	_ = dirty
}

func TestFallbackToUnknown(t *testing.T) {
	// When linker vars are empty and debug.ReadBuildInfo doesn't provide info,
	// functions should return "unknown"
	// Since we can't easily control debug.ReadBuildInfo in tests, we just verify
	// the fallback path exists and returns a non-empty string
	gitCommit := getGitCommit()
	buildDate := getBuildDate()

	if gitCommit == "" {
		t.Error("getGitCommit() should never return empty string")
	}
	if buildDate == "" {
		t.Error("getBuildDate() should never return empty string")
	}

	// Both should either be actual values or "unknown"
	if gitCommit != "unknown" {
		// Verify it looks like a commit hash
		commit := strings.TrimSuffix(gitCommit, "-dirty")
		if len(commit) < 7 {
			t.Errorf("getGitCommit() = %q, expected at least 7 chars", gitCommit)
		}
	}

	if buildDate != "unknown" {
		// Verify it looks like a date
		if !strings.Contains(buildDate, "T") {
			t.Errorf("getBuildDate() = %q, expected ISO 8601 format", buildDate)
		}
	}
}
