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

func TestProtobufChunker_NestedMessages(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Outer {
  message Inner {
    string name = 1;
  }

  Inner inner = 1;
  string id = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least one chunk with Outer message
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}

	// Find the Outer message chunk
	var outerChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.MessageName == "Outer" {
			outerChunk = &result.Chunks[i]
			break
		}
	}

	if outerChunk == nil {
		t.Fatal("expected to find Outer message chunk")
	}

	// Nested message should be within the parent
	if !strings.Contains(outerChunk.Content, "message Inner") {
		t.Error("expected outer chunk to contain nested message")
	}
}

func TestProtobufChunker_OneofFields(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Result {
  oneof result {
    string success = 1;
    string error = 2;
  }
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the message chunk
	var msgChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.MessageName == "Result" {
			msgChunk = &result.Chunks[i]
			break
		}
	}

	if msgChunk == nil {
		t.Fatal("expected to find Result message chunk")
	}

	// Oneof should be preserved
	if !strings.Contains(msgChunk.Content, "oneof result") {
		t.Error("expected chunk to contain oneof")
	}
}

func TestProtobufChunker_MapFields(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Inventory {
  map<string, int32> items = 1;
  map<int64, string> labels = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the message chunk
	var msgChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.MessageName == "Inventory" {
			msgChunk = &result.Chunks[i]
			break
		}
	}

	if msgChunk == nil {
		t.Fatal("expected to find Inventory message chunk")
	}

	// Map fields should be preserved
	if !strings.Contains(msgChunk.Content, "map<string, int32>") {
		t.Error("expected chunk to contain map field")
	}
}

func TestProtobufChunker_ReservedFields(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message User {
  reserved 2, 15, 9 to 11;
  reserved "foo", "bar";
  string id = 1;
  string name = 3;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the message chunk
	var msgChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.MessageName == "User" {
			msgChunk = &result.Chunks[i]
			break
		}
	}

	if msgChunk == nil {
		t.Fatal("expected to find User message chunk")
	}

	// Reserved should be preserved
	if !strings.Contains(msgChunk.Content, "reserved") {
		t.Error("expected chunk to contain reserved")
	}
}

func TestProtobufChunker_StreamingRPCs(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Request {}
message Response {}

service StreamService {
  rpc ServerStream(Request) returns (stream Response);
  rpc ClientStream(stream Request) returns (Response);
  rpc BidiStream(stream Request) returns (stream Response);
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the service chunk
	var svcChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.TypeKind == "service" {
			svcChunk = &result.Chunks[i]
			break
		}
	}

	if svcChunk == nil {
		t.Fatal("expected to find service chunk")
	}

	// Streaming keywords should be preserved
	if !strings.Contains(svcChunk.Content, "stream Response") {
		t.Error("expected chunk to contain stream Response")
	}
	if !strings.Contains(svcChunk.Content, "stream Request") {
		t.Error("expected chunk to contain stream Request")
	}
}

func TestProtobufChunker_EmptyService(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

service EmptyService {
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still parse the empty service
	hasService := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil && chunk.Metadata.Schema.TypeKind == "service" {
			hasService = true
			break
		}
	}
	if !hasService {
		t.Error("expected to find service chunk even if empty")
	}
}

func TestProtobufChunker_PackageWithDots(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

package com.example.api.v1;

message User {
  string id = 1;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Preamble should contain the dotted package
	hasPreamble := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil && chunk.Metadata.Schema.TypeKind == "preamble" {
			hasPreamble = true
			if !strings.Contains(chunk.Content, "com.example.api.v1") {
				t.Error("expected preamble to contain dotted package name")
			}
			break
		}
	}
	if !hasPreamble {
		t.Error("expected to find preamble chunk")
	}
}

func TestProtobufChunker_MultipleEnums(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
}

enum Priority {
  PRIORITY_UNSPECIFIED = 0;
  PRIORITY_HIGH = 1;
  PRIORITY_LOW = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have preamble + 2 enums = 3 chunks
	enumCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil && chunk.Metadata.Schema.TypeKind == "enum" {
			enumCount++
		}
	}

	if enumCount != 2 {
		t.Errorf("expected 2 enum chunks, got %d", enumCount)
	}
}

func TestProtobufChunker_MalformedProtobuf(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Incomplete {
  string id = 1
  // Missing semicolon
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back and produce chunks
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk from fallback, got %d", result.TotalChunks)
	}

	// Should have warnings
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for malformed protobuf")
	}
}

func TestProtobufChunker_OptionsOnMessage(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

import "google/protobuf/descriptor.proto";

message User {
  option deprecated = true;
  string id = 1 [deprecated = true];
  string name = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the message chunk
	var msgChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.MessageName == "User" {
			msgChunk = &result.Chunks[i]
			break
		}
	}

	if msgChunk == nil {
		t.Fatal("expected to find User message chunk")
	}

	// Options should be preserved
	if !strings.Contains(msgChunk.Content, "deprecated") {
		t.Error("expected chunk to contain deprecated option")
	}
}

func TestProtobufChunker_OriginalSizeTracked(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";
message Test {}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OriginalSize != len(content) {
		t.Errorf("expected OriginalSize %d, got %d", len(content), result.OriginalSize)
	}
}

func TestProtobufChunker_TokenEstimatePopulated(t *testing.T) {
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

	for _, chunk := range result.Chunks {
		if chunk.Metadata.TokenEstimate <= 0 {
			t.Error("expected TokenEstimate to be positive")
		}
	}
}

func TestProtobufChunker_ChunkIndexes(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message A {}
message B {}
message C {}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify chunk indexes are sequential starting from 0
	for i, chunk := range result.Chunks {
		if chunk.Index != i {
			t.Errorf("expected chunk index %d, got %d", i, chunk.Index)
		}
	}
}

func TestProtobufChunker_StartEndOffsets(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";
message Test {}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, chunk := range result.Chunks {
		if chunk.EndOffset <= chunk.StartOffset {
			t.Errorf("expected EndOffset > StartOffset, got StartOffset=%d EndOffset=%d",
				chunk.StartOffset, chunk.EndOffset)
		}
	}
}

func TestProtobufChunker_LargeMessageSplitting(t *testing.T) {
	c := NewProtobufChunker()

	// Create a message with many fields
	var largeMessage strings.Builder
	largeMessage.WriteString("syntax = \"proto3\";\n\nmessage LargeMessage {\n")
	for i := 1; i <= 50; i++ {
		largeMessage.WriteString("  string field_" + string(rune('a'+i%26)) + " = " + string(rune('0'+i%10)) + ";\n")
	}
	largeMessage.WriteString("}\n")

	content := largeMessage.String()

	// Use a small max chunk size to trigger splitting
	opts := ChunkOptions{MaxChunkSize: 300}

	result, err := c.Chunk(context.Background(), []byte(content), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be split into multiple chunks
	if result.TotalChunks < 2 {
		t.Errorf("expected content to be split into multiple chunks, got %d", result.TotalChunks)
	}
}

func TestProtobufChunker_ImportStatements(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

import "google/protobuf/timestamp.proto";
import public "google/protobuf/any.proto";
import weak "google/protobuf/empty.proto";

package mypackage;

message MyMessage {
  google.protobuf.Timestamp created_at = 1;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find preamble
	var preambleChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.TypeKind == "preamble" {
			preambleChunk = &result.Chunks[i]
			break
		}
	}

	if preambleChunk == nil {
		t.Fatal("expected to find preamble chunk")
	}

	// All import types should be in preamble
	if !strings.Contains(preambleChunk.Content, "import \"google/protobuf/timestamp.proto\"") {
		t.Error("expected preamble to contain regular import")
	}
}

func TestProtobufChunker_RepeatedFields(t *testing.T) {
	c := NewProtobufChunker()
	content := `syntax = "proto3";

message Container {
  repeated string items = 1;
  repeated int32 numbers = 2;
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the message chunk
	var msgChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Schema != nil &&
			result.Chunks[i].Metadata.Schema.MessageName == "Container" {
			msgChunk = &result.Chunks[i]
			break
		}
	}

	if msgChunk == nil {
		t.Fatal("expected to find Container message chunk")
	}

	// Repeated keyword should be preserved
	if !strings.Contains(msgChunk.Content, "repeated") {
		t.Error("expected chunk to contain repeated keyword")
	}
}
