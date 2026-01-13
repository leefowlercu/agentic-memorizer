package integrations

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// PluginFile defines a plugin file to install.
type PluginFile struct {
	// SourcePath is the path to the source file (or embedded content key).
	SourcePath string
	// TargetName is the filename in the plugin directory.
	TargetName string
	// Content is the file content (if not from source path).
	Content []byte
	// Mode is the file permissions.
	Mode os.FileMode
}

// PluginIntegration is the base type for plugin-based integrations.
type PluginIntegration struct {
	name        string
	harness     string
	description string
	binaryName  string
	pluginDir   string
	pluginFiles []PluginFile
}

// NewPluginIntegration creates a new plugin-based integration.
func NewPluginIntegration(
	name string,
	harness string,
	description string,
	binaryName string,
	pluginDir string,
	pluginFiles []PluginFile,
) *PluginIntegration {
	return &PluginIntegration{
		name:        name,
		harness:     harness,
		description: description,
		binaryName:  binaryName,
		pluginDir:   pluginDir,
		pluginFiles: pluginFiles,
	}
}

// Name returns the integration name.
func (p *PluginIntegration) Name() string {
	return p.name
}

// Harness returns the target harness name.
func (p *PluginIntegration) Harness() string {
	return p.harness
}

// Type returns the integration type.
func (p *PluginIntegration) Type() IntegrationType {
	return IntegrationTypePlugin
}

// Description returns the integration description.
func (p *PluginIntegration) Description() string {
	return p.description
}

// Validate checks if the integration can be set up.
func (p *PluginIntegration) Validate() error {
	// Check if harness binary exists
	if _, found := FindBinary(p.binaryName); !found {
		return fmt.Errorf("%s binary not found in PATH", p.binaryName)
	}

	return nil
}

// Setup installs the plugin integration.
func (p *PluginIntegration) Setup(ctx context.Context) error {
	if err := p.Validate(); err != nil {
		return err
	}

	expandedDir := expandPath(p.pluginDir)

	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(expandedDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory; %w", err)
	}

	// Install each plugin file
	var installedFiles []string
	for _, pf := range p.pluginFiles {
		targetPath := filepath.Join(expandedDir, pf.TargetName)

		// Backup existing file
		if _, err := os.Stat(targetPath); err == nil {
			backupPath := targetPath + ".backup"
			if err := copyFile(targetPath, backupPath); err != nil {
				// Rollback installed files
				for _, f := range installedFiles {
					_ = os.Remove(f)
				}
				return fmt.Errorf("failed to backup existing plugin file; %w", err)
			}
		}

		// Install the file
		if err := p.installFile(pf, targetPath); err != nil {
			// Rollback installed files
			for _, f := range installedFiles {
				_ = os.Remove(f)
			}
			return fmt.Errorf("failed to install plugin file %s; %w", pf.TargetName, err)
		}

		installedFiles = append(installedFiles, targetPath)
	}

	return nil
}

// Teardown removes the plugin integration.
func (p *PluginIntegration) Teardown(ctx context.Context) error {
	expandedDir := expandPath(p.pluginDir)

	// Remove each plugin file
	for _, pf := range p.pluginFiles {
		targetPath := filepath.Join(expandedDir, pf.TargetName)

		// Check if backup exists and restore it
		backupPath := targetPath + ".backup"
		if _, err := os.Stat(backupPath); err == nil {
			if err := os.Rename(backupPath, targetPath); err != nil {
				return fmt.Errorf("failed to restore backup for %s; %w", pf.TargetName, err)
			}
		} else {
			// Remove the file
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove plugin file %s; %w", pf.TargetName, err)
			}
		}
	}

	return nil
}

// IsInstalled checks if the plugin integration is installed.
func (p *PluginIntegration) IsInstalled() (bool, error) {
	expandedDir := expandPath(p.pluginDir)

	// Check if all plugin files exist
	for _, pf := range p.pluginFiles {
		targetPath := filepath.Join(expandedDir, pf.TargetName)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			return false, nil
		}
	}

	return true, nil
}

// Status returns the integration status.
func (p *PluginIntegration) Status() (*StatusInfo, error) {
	// Check if harness is installed
	binaryPath, found := FindBinary(p.binaryName)
	if !found {
		return &StatusInfo{
			Status:  StatusMissingHarness,
			Message: fmt.Sprintf("%s binary not found", p.binaryName),
		}, nil
	}

	expandedDir := expandPath(p.pluginDir)

	// Check if plugin directory exists
	if _, err := os.Stat(expandedDir); os.IsNotExist(err) {
		return &StatusInfo{
			Status:  StatusNotInstalled,
			Message: "Plugin directory does not exist",
		}, nil
	}

	// Check if all plugin files exist
	var missingFiles []string
	var latestModTime time.Time
	for _, pf := range p.pluginFiles {
		targetPath := filepath.Join(expandedDir, pf.TargetName)
		info, err := os.Stat(targetPath)
		if os.IsNotExist(err) {
			missingFiles = append(missingFiles, pf.TargetName)
		} else if err == nil && info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
		}
	}

	if len(missingFiles) > 0 {
		return &StatusInfo{
			Status:     StatusNotInstalled,
			Message:    fmt.Sprintf("Missing plugin files: %v", missingFiles),
			ConfigPath: expandedDir,
		}, nil
	}

	return &StatusInfo{
		Status:      StatusInstalled,
		Message:     fmt.Sprintf("Plugin installed via %s", binaryPath),
		ConfigPath:  expandedDir,
		InstalledAt: latestModTime,
	}, nil
}

// installFile installs a single plugin file.
func (p *PluginIntegration) installFile(pf PluginFile, targetPath string) error {
	mode := pf.Mode
	if mode == 0 {
		mode = 0644
	}

	// If content is provided, write it directly
	if len(pf.Content) > 0 {
		return os.WriteFile(targetPath, pf.Content, mode)
	}

	// If source path is provided, copy the file
	if pf.SourcePath != "" {
		expandedSource := expandPath(pf.SourcePath)
		return copyFileWithMode(expandedSource, targetPath, mode)
	}

	return fmt.Errorf("plugin file %s has no content or source path", pf.TargetName)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	return copyFileWithMode(src, dst, 0)
}

// copyFileWithMode copies a file from src to dst with specified permissions.
func copyFileWithMode(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if mode == 0 {
		info, err := srcFile.Stat()
		if err != nil {
			return err
		}
		mode = info.Mode()
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
