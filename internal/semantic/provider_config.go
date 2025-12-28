package semantic

// ProviderConfig contains shared configuration for all semantic analysis providers.
// Provider-specific implementations extract the fields they need.
type ProviderConfig struct {
	// API credentials
	APIKey string

	// Model selection
	Model string

	// Request configuration
	MaxTokens int
	Timeout   int // seconds

	// Feature flags
	EnableVision bool

	// Analysis constraints
	MaxFileSize int64
}
