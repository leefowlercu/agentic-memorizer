package embeddings

// ProviderConfig contains shared configuration for all embedding providers.
// Provider-specific implementations extract the fields they need.
type ProviderConfig struct {
	// API credentials
	APIKey string

	// Model selection
	Model string

	// Vector dimensions (must match model)
	Dimensions int
}
