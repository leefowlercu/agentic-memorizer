package handlers

// DefaultRegistry creates a registry with all default handlers configured.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Register handlers in order of specificity
	// More specific handlers should be registered first

	// Structured data (should match before text handler)
	r.Register(NewStructuredDataHandler())

	// Documents
	r.Register(NewPDFHandler())
	r.Register(NewRichDocumentHandler())

	// Media
	r.Register(NewImageHandler())

	// Archives
	r.Register(NewArchiveHandler())

	// Text (handles most programming files and plain text)
	r.Register(NewTextHandler())

	// Fallback for anything else
	r.SetFallback(NewUnsupportedHandler())

	return r
}

// NewDefaultTextHandler creates a TextHandler with default configuration.
func NewDefaultTextHandler() *TextHandler {
	return NewTextHandler()
}

// NewDefaultImageHandler creates an ImageHandler with default configuration.
func NewDefaultImageHandler() *ImageHandler {
	return NewImageHandler()
}

// NewDefaultPDFHandler creates a PDFHandler with default configuration.
func NewDefaultPDFHandler() *PDFHandler {
	return NewPDFHandler()
}

// NewDefaultRichDocumentHandler creates a RichDocumentHandler with default configuration.
func NewDefaultRichDocumentHandler() *RichDocumentHandler {
	return NewRichDocumentHandler()
}

// NewDefaultStructuredDataHandler creates a StructuredDataHandler with default configuration.
func NewDefaultStructuredDataHandler() *StructuredDataHandler {
	return NewStructuredDataHandler()
}

// NewDefaultArchiveHandler creates an ArchiveHandler with default configuration.
func NewDefaultArchiveHandler() *ArchiveHandler {
	return NewArchiveHandler()
}

// NewDefaultUnsupportedHandler creates an UnsupportedHandler with default configuration.
func NewDefaultUnsupportedHandler() *UnsupportedHandler {
	return NewUnsupportedHandler()
}
