package logging

// Standard structured logging field names (avoid magic strings)
// Uses dot notation for hierarchical namespacing (OpenTelemetry alignment)
const (
	FieldProcessID     = "process.id"
	FieldSessionID     = "session.id"
	FieldRequestID     = "request.id"
	FieldClientID      = "client.id"
	FieldClientType    = "client.type"
	FieldClientVersion = "client.version"
	FieldComponent     = "component"
	FieldClientName    = "client_name"
	FieldTraceID       = "trace_id" // Future: OpenTelemetry
)

// Component names
const (
	ComponentMCPServer    = "mcp-server"
	ComponentSSEClient    = "sse-client"
	ComponentDaemonSSE    = "daemon-sse"
	ComponentGraphManager = "graph-manager"
)

// HTTP headers for client identification
const (
	HeaderClientID      = "X-Client-ID"
	HeaderClientType    = "X-Client-Type"
	HeaderClientVersion = "X-Client-Version"
)
