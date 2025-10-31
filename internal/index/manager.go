package index

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Manager handles loading, saving, and updating the computed index
type Manager struct {
	currentIndex  *types.Index
	indexPath     string
	indexLock     sync.RWMutex
	buildMetadata BuildMetadata
}

// NewManager creates a new index manager
func NewManager(indexPath string) *Manager {
	return &Manager{
		indexPath: indexPath,
	}
}

// LoadComputed loads the computed index from disk with validation
func (m *Manager) LoadComputed() (*ComputedIndex, error) {
	m.indexLock.RLock()
	defer m.indexLock.RUnlock()

	data, err := os.ReadFile(m.indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file: %w", err)
	}

	var computed ComputedIndex
	if err := json.Unmarshal(data, &computed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index (corrupt): %w", err)
	}

	// Validate index structure
	if computed.Index == nil {
		return nil, fmt.Errorf("corrupt index: missing index data")
	}
	if computed.Version == "" {
		return nil, fmt.Errorf("corrupt index: missing version")
	}
	if computed.Index.Entries == nil {
		// Initialize empty entries slice if nil
		computed.Index.Entries = []types.IndexEntry{}
	}

	// Update internal state
	m.currentIndex = computed.Index
	m.buildMetadata = computed.Metadata

	return &computed, nil
}

// SetIndex sets the current index (used by daemon when building)
func (m *Manager) SetIndex(index *types.Index, metadata BuildMetadata) {
	m.indexLock.Lock()
	defer m.indexLock.Unlock()

	m.currentIndex = index
	m.buildMetadata = metadata
}

// WriteAtomic writes the computed index to disk atomically
// Uses temp file + sync + atomic rename to ensure consistency
func (m *Manager) WriteAtomic(daemonVersion string) error {
	m.indexLock.RLock()
	defer m.indexLock.RUnlock()

	if m.currentIndex == nil {
		return fmt.Errorf("no index to write")
	}

	computed := ComputedIndex{
		Version:       "1.0",
		GeneratedAt:   time.Now(),
		DaemonVersion: daemonVersion,
		Index:         m.currentIndex,
		Metadata:      m.buildMetadata,
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(computed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write to temp file
	tmpPath := m.indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk to ensure data is written
	f, err := os.Open(tmpPath)
	if err == nil {
		f.Sync()
		f.Close()
	}

	// Atomic rename
	if err := os.Rename(tmpPath, m.indexPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// GetCurrent returns the current index (thread-safe read)
func (m *Manager) GetCurrent() *types.Index {
	m.indexLock.RLock()
	defer m.indexLock.RUnlock()
	return m.currentIndex
}

// UpdateSingle updates a single entry in the index (for Phase 2)
// This is a placeholder for incremental updates
func (m *Manager) UpdateSingle(entry types.IndexEntry) error {
	m.indexLock.Lock()
	defer m.indexLock.Unlock()

	if m.currentIndex == nil {
		return fmt.Errorf("no index loaded")
	}

	// Find and update existing entry or append new one
	updated := false
	for i, e := range m.currentIndex.Entries {
		if e.Metadata.Path == entry.Metadata.Path {
			m.currentIndex.Entries[i] = entry
			updated = true
			break
		}
	}

	if !updated {
		m.currentIndex.Entries = append(m.currentIndex.Entries, entry)
		m.currentIndex.Stats.TotalFiles++
	}

	m.currentIndex.Generated = time.Now()
	return nil
}

// RemoveFile removes a file entry from the index (for Phase 2)
func (m *Manager) RemoveFile(path string) error {
	m.indexLock.Lock()
	defer m.indexLock.Unlock()

	if m.currentIndex == nil {
		return fmt.Errorf("no index loaded")
	}

	// Find and remove the entry
	for i, e := range m.currentIndex.Entries {
		if e.Metadata.Path == path {
			// Remove entry by slicing
			m.currentIndex.Entries = append(
				m.currentIndex.Entries[:i],
				m.currentIndex.Entries[i+1:]...,
			)
			m.currentIndex.Stats.TotalFiles--
			m.currentIndex.Generated = time.Now()
			return nil
		}
	}

	return fmt.Errorf("file not found in index: %s", path)
}
