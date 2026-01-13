package envelopes

// NoneEnvelope is a passthrough envelope that doesn't wrap content.
type NoneEnvelope struct{}

// NewNoneEnvelope creates a new none envelope.
func NewNoneEnvelope() *NoneEnvelope {
	return &NoneEnvelope{}
}

// Name returns the envelope name.
func (e *NoneEnvelope) Name() string {
	return "none"
}

// ContentType returns the MIME content type.
func (e *NoneEnvelope) ContentType() string {
	return "application/octet-stream"
}

// Description returns a human-readable description.
func (e *NoneEnvelope) Description() string {
	return "No envelope wrapping (raw output)"
}

// Wrap returns the content unchanged.
func (e *NoneEnvelope) Wrap(content []byte, stats *ExportStats) ([]byte, error) {
	return content, nil
}
