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
	if oldCfg.Memory.Root != newCfg.Memory.Root {
		errors = append(errors, fmt.Sprintf(
			"memory.root cannot be changed during reload (old: %s, new: %s) - restart daemon required",
			oldCfg.Memory.Root, newCfg.Memory.Root))
	}

	if oldCfg.Semantic.CacheDir != newCfg.Semantic.CacheDir {
		errors = append(errors, fmt.Sprintf(
			"semantic.cache_dir cannot be changed during reload (old: %s, new: %s) - restart daemon required",
			oldCfg.Semantic.CacheDir, newCfg.Semantic.CacheDir))
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
