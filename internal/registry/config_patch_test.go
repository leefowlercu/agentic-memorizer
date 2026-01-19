package registry

import "testing"

func TestPathConfigPatch_IsEmpty(t *testing.T) {
	if !(*PathConfigPatch)(nil).IsEmpty() {
		t.Fatal("expected nil patch to be empty")
	}

	patch := &PathConfigPatch{}
	if !patch.IsEmpty() {
		t.Fatal("expected empty patch to be empty")
	}

	value := true
	patch.SkipHidden = &value
	if patch.IsEmpty() {
		t.Fatal("expected patch with fields to be non-empty")
	}
}

func TestApplyPathConfigPatch(t *testing.T) {
	base := &PathConfig{
		SkipExtensions:  []string{".exe"},
		SkipDirectories: []string{"node_modules"},
		SkipFiles:       []string{"Thumbs.db"},
		SkipHidden:      true,
		IncludeFiles:    []string{".env"},
	}

	patch := &PathConfigPatch{
		SkipHidden:         boolPtrPatch(false),
		SetSkipExtensions:  []string{".only"},
		AddSkipDirectories: []string{"vendor"},
		AddIncludeFiles:    []string{".envrc"},
	}

	got := ApplyPathConfigPatch(base, patch)
	if got.SkipHidden {
		t.Error("expected SkipHidden to be false")
	}
	if len(got.SkipExtensions) != 1 || got.SkipExtensions[0] != ".only" {
		t.Fatalf("expected skip extensions to be replaced, got %v", got.SkipExtensions)
	}
	if len(got.SkipDirectories) != 2 {
		t.Fatalf("expected skip directories to include vendor, got %v", got.SkipDirectories)
	}
	if len(got.IncludeFiles) != 2 {
		t.Fatalf("expected include files to merge, got %v", got.IncludeFiles)
	}
}

func TestMergeUnique(t *testing.T) {
	tests := []struct {
		name      string
		base      []string
		additions []string
		want      []string
	}{
		{
			name:      "empty slices",
			base:      nil,
			additions: nil,
			want:      []string{},
		},
		{
			name:      "only base",
			base:      []string{"a", "b"},
			additions: nil,
			want:      []string{"a", "b"},
		},
		{
			name:      "only additions",
			base:      nil,
			additions: []string{"c", "d"},
			want:      []string{"c", "d"},
		},
		{
			name:      "no overlap",
			base:      []string{"a", "b"},
			additions: []string{"c", "d"},
			want:      []string{"a", "b", "c", "d"},
		},
		{
			name:      "with overlap",
			base:      []string{"a", "b", "c"},
			additions: []string{"b", "c", "d"},
			want:      []string{"a", "b", "c", "d"},
		},
		{
			name:      "duplicates in base",
			base:      []string{"a", "a", "b"},
			additions: []string{"c"},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "duplicates in additions",
			base:      []string{"a"},
			additions: []string{"b", "b", "c", "c"},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "duplicates in both",
			base:      []string{"a", "a", "b"},
			additions: []string{"b", "c", "c"},
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "all same values",
			base:      []string{"a", "a"},
			additions: []string{"a", "a"},
			want:      []string{"a"},
		},
		{
			name:      "empty strings",
			base:      []string{"", "a"},
			additions: []string{"b", ""},
			want:      []string{"", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeUnique(tt.base, tt.additions)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d items, got %d", len(tt.want), len(got))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("item %d: expected %q, got %q", i, tt.want[i], v)
				}
			}
		})
	}
}

func TestNormalizeExtensions(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "already with dots",
			input: []string{".log", ".tmp"},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "without dots",
			input: []string{"log", "tmp"},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "mixed",
			input: []string{".log", "tmp"},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "with spaces",
			input: []string{" .log ", " tmp "},
			want:  []string{".log", ".tmp"},
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "nil slice",
			input: nil,
			want:  []string{},
		},
		{
			name:  "empty string in slice",
			input: []string{".go", "", ".py"},
			want:  []string{".go", "", ".py"},
		},
		{
			name:  "whitespace only string",
			input: []string{".go", "   ", ".py"},
			want:  []string{".go", "", ".py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeExtensions(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d extensions, got %d", len(tt.want), len(got))
			}
			for i, ext := range got {
				if ext != tt.want[i] {
					t.Errorf("extension %d: expected %q, got %q", i, tt.want[i], ext)
				}
			}
		})
	}
}

func boolPtrPatch(v bool) *bool {
	return &v
}
