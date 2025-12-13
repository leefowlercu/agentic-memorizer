package logging

import "log/slog"

// WithProcessID enriches logger with process_id
func WithProcessID(logger *slog.Logger, processID string) *slog.Logger {
	return logger.With(FieldProcessID, processID)
}

// WithSessionID enriches logger with session_id
func WithSessionID(logger *slog.Logger, sessionID string) *slog.Logger {
	return logger.With(FieldSessionID, sessionID)
}

// WithComponent enriches logger with component name
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	return logger.With(FieldComponent, component)
}

// WithClientInfo enriches logger with client name and version
func WithClientInfo(logger *slog.Logger, name, version string) *slog.Logger {
	return logger.With(
		FieldClientName, name,
		FieldClientVersion, version,
	)
}

// WithMCPProcess enriches logger with process_id + component for MCP server
func WithMCPProcess(logger *slog.Logger, processID string) *slog.Logger {
	return logger.With(
		FieldProcessID, processID,
		FieldComponent, ComponentMCPServer,
	)
}

// WithSSEClient enriches logger with component for SSE client
func WithSSEClient(logger *slog.Logger) *slog.Logger {
	return logger.With(FieldComponent, ComponentSSEClient)
}

// WithDaemonSSE enriches logger with client.id, client.type, and client.version for daemon SSE
func WithDaemonSSE(logger *slog.Logger, clientID, clientType, clientVersion string) *slog.Logger {
	return logger.With(
		FieldClientID, clientID,
		FieldClientType, clientType,
		FieldClientVersion, clientVersion,
		FieldComponent, ComponentDaemonSSE,
	)
}
