package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtobufChunker_Name(t *testing.T) {
	c := NewProtobufChunker()
	if c.Name() != "protobuf" {
		t.Errorf("expected name 'protobuf', got %q", c.Name())
	}
}

func TestProtobufChunker_Priority(t *testing.T) {
	c := NewProtobufChunker()
	if c.Priority() != 42 {
		t.Errorf("expected priority 42, got %d", c.Priority())
	}
}

func TestProtobufChunker_CanHandle(t *testing.T) {
	c := NewProtobufChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-protobuf", "", true},
		{"application/x-protobuf", "", true},
		{"text/protobuf", "", true},
		{"", "sample.proto", true},
		{"", "/path/to/sample.proto", true},
		{"text/plain", "", false},
		{"", "go", false},
		{"", "python", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestProtobufChunker_EmptyContent(t *testing.T) {
	c := NewProtobufChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "protobuf" {
		t.Errorf("expected chunker name 'protobuf', got %q", result.ChunkerUsed)
	}
}

func TestProtobufChunker_SingleMessage(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message User {
  string id = 1;
  string name = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least 1 chunk (may have preamble as separate chunk)
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}

	// Find the message chunk
	var messageChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.TypeKind == "message" {
			messageChunk = &result.Chunks[i]
			break
		}
	}

	if messageChunk == nil {
		t.Fatal("expected to find a message chunk")
	}

	if messageChunk.Metadata.Schema.MessageName != "User" {
		t.Errorf("expected message name 'User', got %q", messageChunk.Metadata.Schema.MessageName)
	}
}

func TestProtobufChunker_Enum(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
  STATUS_INACTIVE = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the enum chunk
	var enumChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.TypeKind == "enum" {
			enumChunk = &result.Chunks[i]
			break
		}
	}

	if enumChunk == nil {
		t.Fatal("expected to find an enum chunk")
	}

	if enumChunk.Metadata.Schema.MessageName != "Status" {
		t.Errorf("expected enum name 'Status', got %q", enumChunk.Metadata.Schema.MessageName)
	}
}

func TestProtobufChunker_Service(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Request {}
message Response {}

service MyService {
  rpc GetData(Request) returns (Response);
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the service chunk
	var serviceChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.TypeKind == "service" {
			serviceChunk = &result.Chunks[i]
			break
		}
	}

	if serviceChunk == nil {
		t.Fatal("expected to find a service chunk")
	}

	if serviceChunk.Metadata.Schema.ServiceName != "MyService" {
		t.Errorf("expected service name 'MyService', got %q", serviceChunk.Metadata.Schema.ServiceName)
	}
}

func TestProtobufChunker_MultipleDefinitions(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

package sample.v1;

message User {
  string id = 1;
}

enum Role {
  ROLE_UNSPECIFIED = 0;
  ROLE_ADMIN = 1;
}

service UserService {
  rpc GetUser(User) returns (User);
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: preamble + message + enum + service = 4 chunks
	if result.TotalChunks != 4 {
		t.Errorf("expected 4 chunks, got %d", result.TotalChunks)
	}

	// Verify we have all types
	typeKinds := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil {
			typeKinds[chunk.Metadata.Schema.TypeKind] = true
		}
	}

	expectedKinds := []string{"preamble", "message", "enum", "service"}
	for _, kind := range expectedKinds {
		if !typeKinds[kind] {
			t.Errorf("expected to find %q type kind", kind)
		}
	}
}

func TestProtobufChunker_TestdataFixture(t *testing.T) {
	c := NewProtobufChunker()

	// Read the testdata fixture
	fixturePath := filepath.Join("..", "..", "testdata", "devops", "sample.proto")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has: preamble + User message + Role enum + multiple request/response messages + service
	if result.TotalChunks < 5 {
		t.Errorf("expected at least 5 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify we have a service
	hasService := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil && chunk.Metadata.Schema.TypeKind == "service" {
			hasService = true
			break
		}
	}
	if !hasService {
		t.Error("expected to find a service chunk in fixture")
	}
}

func TestProtobufChunker_ContextCancellation(t *testing.T) {
	c := NewProtobufChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `syntax = "proto3";
message Test {}
`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestProtobufChunker_ChunkType(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";
message Test {}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Type != ChunkTypeStructured {
			t.Errorf("expected ChunkTypeStructured, got %q", chunk.Metadata.Type)
		}
	}
}

func TestProtobufChunker_CommentsPreserved(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

// This is a comment for User
message User {
  string id = 1;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the message chunk
	var messageChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.TypeKind == "message" {
			messageChunk = &result.Chunks[i]
			break
		}
	}

	if messageChunk == nil {
		t.Fatal("expected to find a message chunk")
	}

	// Check that the comment is included
	if !strings.Contains(messageChunk.Content, "This is a comment") {
		t.Error("expected chunk to contain comment")
	}
}
