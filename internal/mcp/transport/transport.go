package transport

// Transport defines the interface for MCP communication layers
type Transport interface {
	// Read reads a single JSON-RPC message
	Read() ([]byte, error)

	// Write writes a JSON-RPC message
	Write(data []byte) error

	// Close closes the transport
	Close() error
}
