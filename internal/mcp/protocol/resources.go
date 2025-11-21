package protocol

// Resource definition
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListResponse is the response for resources/list
type ResourcesListResponse struct {
	Resources []Resource `json:"resources"`
}

// ResourcesReadRequest is the request for resources/read
type ResourcesReadRequest struct {
	URI string `json:"uri"`
}

// ResourcesReadResponse is the response for resources/read
type ResourcesReadResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // Base64-encoded binary data
}
