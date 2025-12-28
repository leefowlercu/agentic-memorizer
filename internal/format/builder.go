package format

// BuilderType represents the type of output structure being built
type BuilderType string

const (
	// BuilderTypeSection represents hierarchical key-value pairs
	BuilderTypeSection BuilderType = "section"

	// BuilderTypeTable represents columnar data
	BuilderTypeTable BuilderType = "table"

	// BuilderTypeList represents ordered or unordered lists
	BuilderTypeList BuilderType = "list"

	// BuilderTypeProgress represents progress indicators
	BuilderTypeProgress BuilderType = "progress"

	// BuilderTypeStatus represents status messages
	BuilderTypeStatus BuilderType = "status"

	// BuilderTypeError represents structured error messages
	BuilderTypeError BuilderType = "error"

	// BuilderTypeFiles represents files index data
	BuilderTypeFiles BuilderType = "files"

	// BuilderTypeFacts represents facts index data
	BuilderTypeFacts BuilderType = "facts"
)

// String returns the string representation of the builder type
func (bt BuilderType) String() string {
	return string(bt)
}

// Buildable represents any structure that can be formatted
type Buildable interface {
	// Type returns the builder type
	Type() BuilderType

	// Validate checks if the builder is correctly constructed
	Validate() error
}
