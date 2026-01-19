package registry

import "strings"

// PathConfigPatch describes incremental updates to a PathConfig.
type PathConfigPatch struct {
	SkipHidden *bool `json:"skip_hidden,omitempty"`
	UseVision  *bool `json:"use_vision,omitempty"`

	SetSkipExtensions []string `json:"set_skip_extensions,omitempty"`
	AddSkipExtensions []string `json:"add_skip_extensions,omitempty"`

	SetSkipDirectories []string `json:"set_skip_directories,omitempty"`
	AddSkipDirectories []string `json:"add_skip_directories,omitempty"`

	SetSkipFiles []string `json:"set_skip_files,omitempty"`
	AddSkipFiles []string `json:"add_skip_files,omitempty"`

	AddIncludeExtensions  []string `json:"add_include_extensions,omitempty"`
	AddIncludeDirectories []string `json:"add_include_directories,omitempty"`
	AddIncludeFiles       []string `json:"add_include_files,omitempty"`
}

// IsEmpty returns true if the patch has no changes.
func (p *PathConfigPatch) IsEmpty() bool {
	if p == nil {
		return true
	}
	return p.SkipHidden == nil &&
		p.UseVision == nil &&
		len(p.SetSkipExtensions) == 0 &&
		len(p.AddSkipExtensions) == 0 &&
		len(p.SetSkipDirectories) == 0 &&
		len(p.AddSkipDirectories) == 0 &&
		len(p.SetSkipFiles) == 0 &&
		len(p.AddSkipFiles) == 0 &&
		len(p.AddIncludeExtensions) == 0 &&
		len(p.AddIncludeDirectories) == 0 &&
		len(p.AddIncludeFiles) == 0
}

// ApplyPathConfigPatch applies a patch to a base config and returns a new config.
func ApplyPathConfigPatch(base *PathConfig, patch *PathConfigPatch) *PathConfig {
	cfg := base.Clone()
	if cfg == nil {
		cfg = &PathConfig{}
	}
	if patch == nil {
		return cfg
	}

	if patch.SkipHidden != nil {
		cfg.SkipHidden = *patch.SkipHidden
	}
	if patch.UseVision != nil {
		cfg.UseVision = patch.UseVision
	}

	if len(patch.SetSkipExtensions) > 0 {
		cfg.SkipExtensions = normalizeExtensions(patch.SetSkipExtensions)
	} else if len(patch.AddSkipExtensions) > 0 {
		cfg.SkipExtensions = mergeUnique(cfg.SkipExtensions, normalizeExtensions(patch.AddSkipExtensions))
	}

	if len(patch.SetSkipDirectories) > 0 {
		cfg.SkipDirectories = patch.SetSkipDirectories
	} else if len(patch.AddSkipDirectories) > 0 {
		cfg.SkipDirectories = mergeUnique(cfg.SkipDirectories, patch.AddSkipDirectories)
	}

	if len(patch.SetSkipFiles) > 0 {
		cfg.SkipFiles = patch.SetSkipFiles
	} else if len(patch.AddSkipFiles) > 0 {
		cfg.SkipFiles = mergeUnique(cfg.SkipFiles, patch.AddSkipFiles)
	}

	if len(patch.AddIncludeExtensions) > 0 {
		cfg.IncludeExtensions = mergeUnique(cfg.IncludeExtensions, normalizeExtensions(patch.AddIncludeExtensions))
	}
	if len(patch.AddIncludeDirectories) > 0 {
		cfg.IncludeDirectories = mergeUnique(cfg.IncludeDirectories, patch.AddIncludeDirectories)
	}
	if len(patch.AddIncludeFiles) > 0 {
		cfg.IncludeFiles = mergeUnique(cfg.IncludeFiles, patch.AddIncludeFiles)
	}

	return cfg
}

func mergeUnique(base []string, additions []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(base)+len(additions))
	for _, item := range base {
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, item := range additions {
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func normalizeExtensions(exts []string) []string {
	out := make([]string, 0, len(exts))
	for _, ext := range exts {
		ext = strings.TrimSpace(ext)
		ext = strings.ToLower(ext)
		if ext != "" && !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		out = append(out, ext)
	}
	return out
}
