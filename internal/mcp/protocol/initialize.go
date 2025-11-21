package protocol

// Initialize Request (client → server)
type InitializeRequest struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

type ClientCapabilities struct {
	Sampling    *SamplingCapability    `json:"sampling,omitempty"`
	Roots       *RootsCapability       `json:"roots,omitempty"`
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`
}

type SamplingCapability struct{}    // Placeholder, not used in Phase 1
type RootsCapability struct{}       // Placeholder, not used in Phase 1
type ElicitationCapability struct{} // Placeholder, not used in Phase 1

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Initialize Response (server → client)
type InitializeResponse struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`   // Phase 5
	ListChanged bool `json:"listChanged,omitempty"` // Phase 5
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // Phase 5
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // Phase 5
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Initialized Notification (client → server)
type InitializedNotification struct {
	// Empty params for now
}
