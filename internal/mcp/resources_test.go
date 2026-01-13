package mcp

import (
	"testing"
)

func TestAvailableResources(t *testing.T) {
	resources := AvailableResources()

	if len(resources) != 4 {
		t.Errorf("AvailableResources returned %d resources, want 4", len(resources))
	}

	// Verify each resource has required fields
	for _, r := range resources {
		if r.URI == "" {
			t.Error("Resource has empty URI")
		}
		if r.Name == "" {
			t.Error("Resource has empty Name")
		}
		if r.MIMEType == "" {
			t.Error("Resource has empty MIMEType")
		}
		if r.Format == "" {
			t.Error("Resource has empty Format")
		}
	}
}

func TestGetResourceFormat(t *testing.T) {
	tests := []struct {
		uri      string
		format   string
		mimeType string
		ok       bool
	}{
		{ResourceURIIndex, "xml", "application/xml", true},
		{ResourceURIIndexXML, "xml", "application/xml", true},
		{ResourceURIIndexJSON, "json", "application/json", true},
		{ResourceURIIndexTOON, "toon", "text/plain", true},
		{"memorizer://invalid", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range tests {
		format, mimeType, ok := GetResourceFormat(tt.uri)

		if ok != tt.ok {
			t.Errorf("GetResourceFormat(%q) ok = %v, want %v", tt.uri, ok, tt.ok)
		}
		if format != tt.format {
			t.Errorf("GetResourceFormat(%q) format = %q, want %q", tt.uri, format, tt.format)
		}
		if mimeType != tt.mimeType {
			t.Errorf("GetResourceFormat(%q) mimeType = %q, want %q", tt.uri, mimeType, tt.mimeType)
		}
	}
}

func TestIsValidResourceURI(t *testing.T) {
	tests := []struct {
		uri   string
		valid bool
	}{
		{ResourceURIIndex, true},
		{ResourceURIIndexXML, true},
		{ResourceURIIndexJSON, true},
		{ResourceURIIndexTOON, true},
		{"memorizer://invalid", false},
		{"", false},
		{"http://example.com", false},
	}

	for _, tt := range tests {
		valid := IsValidResourceURI(tt.uri)
		if valid != tt.valid {
			t.Errorf("IsValidResourceURI(%q) = %v, want %v", tt.uri, valid, tt.valid)
		}
	}
}

func TestResourceURIConstants(t *testing.T) {
	// Verify URI format consistency
	if ResourceURIIndex != "memorizer://index" {
		t.Errorf("ResourceURIIndex = %q, want %q", ResourceURIIndex, "memorizer://index")
	}
	if ResourceURIIndexXML != "memorizer://index/xml" {
		t.Errorf("ResourceURIIndexXML = %q, want %q", ResourceURIIndexXML, "memorizer://index/xml")
	}
	if ResourceURIIndexJSON != "memorizer://index/json" {
		t.Errorf("ResourceURIIndexJSON = %q, want %q", ResourceURIIndexJSON, "memorizer://index/json")
	}
	if ResourceURIIndexTOON != "memorizer://index/toon" {
		t.Errorf("ResourceURIIndexTOON = %q, want %q", ResourceURIIndexTOON, "memorizer://index/toon")
	}
}
