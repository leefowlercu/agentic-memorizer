package envelopes

// ExportStats contains statistics about an export operation.
// This is a copy to avoid circular imports.
type ExportStats struct {
	FileCount         int
	DirectoryCount    int
	ChunkCount        int
	TagCount          int
	TopicCount        int
	EntityCount       int
	RelationshipCount int
	Format            string
	OutputSize        int
}

// Envelope wraps exported content in a harness-specific format.
type Envelope interface {
	// Wrap wraps the content with envelope-specific formatting.
	Wrap(content []byte, stats *ExportStats) ([]byte, error)

	// Name returns the envelope name.
	Name() string

	// ContentType returns the MIME content type of the wrapped output.
	ContentType() string

	// Description returns a human-readable description.
	Description() string
}
