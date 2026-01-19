package events

// ConfigReloadEvent contains data for config reload events.
type ConfigReloadEvent struct {
	// ChangedSections lists which config sections were modified.
	ChangedSections []string

	// ReloadableChanges indicates if all changes are hot-reloadable.
	ReloadableChanges bool

	// Error contains the error message if reload failed (for ConfigReloadFailed events).
	Error string
}

// NewConfigReloaded creates a ConfigReloaded event.
func NewConfigReloaded(changedSections []string, reloadable bool) Event {
	return NewEvent(ConfigReloaded, &ConfigReloadEvent{
		ChangedSections:   changedSections,
		ReloadableChanges: reloadable,
	})
}

// NewConfigReloadFailed creates a ConfigReloadFailed event.
func NewConfigReloadFailed(err error) Event {
	return NewEvent(ConfigReloadFailed, &ConfigReloadEvent{
		Error: errorString(err),
	})
}
