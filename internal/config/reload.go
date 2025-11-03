package config

import (
	"fmt"
	"strings"
)

// ValidateReload checks if configuration changes are compatible with hot-reload
// Returns an error if any immutable fields have changed
func ValidateReload(oldCfg, newCfg *Config) error {
	var errors []string

	// Check immutable fields that require daemon restart
	if oldCfg.MemoryRoot != newCfg.MemoryRoot {
		errors = append(errors, fmt.Sprintf(
			"memory_root cannot be changed during reload (old: %s, new: %s) - restart daemon required",
			oldCfg.MemoryRoot, newCfg.MemoryRoot))
	}

	if oldCfg.Analysis.CacheDir != newCfg.Analysis.CacheDir {
		errors = append(errors, fmt.Sprintf(
			"analysis.cache_dir cannot be changed during reload (old: %s, new: %s) - restart daemon required",
			oldCfg.Analysis.CacheDir, newCfg.Analysis.CacheDir))
	}

	if oldCfg.Daemon.LogFile != newCfg.Daemon.LogFile {
		errors = append(errors, fmt.Sprintf(
			"daemon.log_file cannot be changed during reload (old: %s, new: %s) - restart daemon required",
			oldCfg.Daemon.LogFile, newCfg.Daemon.LogFile))
	}

	if len(errors) > 0 {
		return fmt.Errorf("reload validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}
