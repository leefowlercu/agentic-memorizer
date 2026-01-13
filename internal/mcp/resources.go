package mcp

// Resource URIs for the memorizer MCP server.
const (
	// ResourceURIIndex is the default knowledge graph index (XML format).
	ResourceURIIndex = "memorizer://index"

	// ResourceURIIndexXML is the XML-formatted knowledge graph index.
	ResourceURIIndexXML = "memorizer://index/xml"

	// ResourceURIIndexJSON is the JSON-formatted knowledge graph index.
	ResourceURIIndexJSON = "memorizer://index/json"

	// ResourceURIIndexTOON is the TOON-formatted knowledge graph index.
	ResourceURIIndexTOON = "memorizer://index/toon"
)

// ResourceInfo contains metadata about a resource.
type ResourceInfo struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
	Format      string
}

// AvailableResources returns information about all available resources.
func AvailableResources() []ResourceInfo {
	return []ResourceInfo{
		{
			URI:         ResourceURIIndex,
			Name:        "Knowledge Graph Index",
			Description: "The complete memorizer knowledge graph index (default XML format)",
			MIMEType:    "application/xml",
			Format:      "xml",
		},
		{
			URI:         ResourceURIIndexXML,
			Name:        "Knowledge Graph Index (XML)",
			Description: "The memorizer knowledge graph index in XML format",
			MIMEType:    "application/xml",
			Format:      "xml",
		},
		{
			URI:         ResourceURIIndexJSON,
			Name:        "Knowledge Graph Index (JSON)",
			Description: "The memorizer knowledge graph index in JSON format",
			MIMEType:    "application/json",
			Format:      "json",
		},
		{
			URI:         ResourceURIIndexTOON,
			Name:        "Knowledge Graph Index (TOON)",
			Description: "The memorizer knowledge graph index in token-optimized notation (~40% smaller)",
			MIMEType:    "text/plain",
			Format:      "toon",
		},
	}
}

// GetResourceFormat returns the format for a given resource URI.
func GetResourceFormat(uri string) (format string, mimeType string, ok bool) {
	switch uri {
	case ResourceURIIndex, ResourceURIIndexXML:
		return "xml", "application/xml", true
	case ResourceURIIndexJSON:
		return "json", "application/json", true
	case ResourceURIIndexTOON:
		return "toon", "text/plain", true
	default:
		return "", "", false
	}
}

// IsValidResourceURI checks if a URI is a valid memorizer resource.
func IsValidResourceURI(uri string) bool {
	_, _, ok := GetResourceFormat(uri)
	return ok
}
