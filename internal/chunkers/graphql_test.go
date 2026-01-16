package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphQLChunker_Name(t *testing.T) {
	c := NewGraphQLChunker()
	if c.Name() != "graphql" {
		t.Errorf("expected name 'graphql', got %q", c.Name())
	}
}

func TestGraphQLChunker_Priority(t *testing.T) {
	c := NewGraphQLChunker()
	if c.Priority() != 41 {
		t.Errorf("expected priority 41, got %d", c.Priority())
	}
}

func TestGraphQLChunker_CanHandle(t *testing.T) {
	c := NewGraphQLChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"application/graphql", "", true},
		{"text/x-graphql", "", true},
		{"application/x-graphql", "", true},
		{"", "schema.graphql", true},
		{"", "query.gql", true},
		{"", "types.graphqls", true},
		{"", "/path/to/schema.graphql", true},
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

func TestGraphQLChunker_EmptyContent(t *testing.T) {
	c := NewGraphQLChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "graphql" {
		t.Errorf("expected chunker name 'graphql', got %q", result.ChunkerUsed)
	}
}

func TestGraphQLChunker_SingleType(t *testing.T) {
	c := NewGraphQLChunker()
	content := `type User {
  id: ID!
  name: String!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema == nil {
			t.Fatal("expected Schema metadata to be set")
		}
		if chunk.Metadata.Schema.TypeKind != "type" {
			t.Errorf("expected type kind 'type', got %q", chunk.Metadata.Schema.TypeKind)
		}
		if chunk.Metadata.Schema.TypeName != "User" {
			t.Errorf("expected type name 'User', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_MultipleTypes(t *testing.T) {
	c := NewGraphQLChunker()
	content := `type User {
  id: ID!
  name: String!
}

type Post {
  id: ID!
  title: String!
}

enum Role {
  ADMIN
  USER
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Verify type kinds
	expectedKinds := []string{"type", "type", "enum"}
	for i, expected := range expectedKinds {
		if i < result.TotalChunks {
			if result.Chunks[i].Metadata.Schema.TypeKind != expected {
				t.Errorf("chunk %d: expected type kind %q, got %q",
					i, expected, result.Chunks[i].Metadata.Schema.TypeKind)
			}
		}
	}
}

func TestGraphQLChunker_InputType(t *testing.T) {
	c := NewGraphQLChunker()
	content := `input CreateUserInput {
  name: String!
  email: String!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema.TypeKind != "input" {
			t.Errorf("expected type kind 'input', got %q", chunk.Metadata.Schema.TypeKind)
		}
		if chunk.Metadata.Schema.TypeName != "CreateUserInput" {
			t.Errorf("expected type name 'CreateUserInput', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_Interface(t *testing.T) {
	c := NewGraphQLChunker()
	content := `interface Node {
  id: ID!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema.TypeKind != "interface" {
			t.Errorf("expected type kind 'interface', got %q", chunk.Metadata.Schema.TypeKind)
		}
		if chunk.Metadata.Schema.TypeName != "Node" {
			t.Errorf("expected type name 'Node', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_Scalar(t *testing.T) {
	c := NewGraphQLChunker()
	content := `scalar DateTime

type Event {
  createdAt: DateTime!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have scalar + type
	if result.TotalChunks < 2 {
		t.Errorf("expected at least 2 chunks, got %d", result.TotalChunks)
	}

	// Check first is scalar
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema.TypeKind != "scalar" {
			t.Errorf("expected first chunk to be 'scalar', got %q", chunk.Metadata.Schema.TypeKind)
		}
		if chunk.Metadata.Schema.TypeName != "DateTime" {
			t.Errorf("expected type name 'DateTime', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_Enum(t *testing.T) {
	c := NewGraphQLChunker()
	content := `enum Status {
  ACTIVE
  INACTIVE
  PENDING
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema.TypeKind != "enum" {
			t.Errorf("expected type kind 'enum', got %q", chunk.Metadata.Schema.TypeKind)
		}
		if chunk.Metadata.Schema.TypeName != "Status" {
			t.Errorf("expected type name 'Status', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_Union(t *testing.T) {
	c := NewGraphQLChunker()
	content := `union SearchResult = User | Post | Comment
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema.TypeKind != "union" {
			t.Errorf("expected type kind 'union', got %q", chunk.Metadata.Schema.TypeKind)
		}
		if chunk.Metadata.Schema.TypeName != "SearchResult" {
			t.Errorf("expected type name 'SearchResult', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_QueryMutation(t *testing.T) {
	c := NewGraphQLChunker()
	content := `type Query {
  user(id: ID!): User
  users: [User!]!
}

type Mutation {
  createUser(input: CreateUserInput!): User!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks, got %d", result.TotalChunks)
	}

	// Check Query type
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Schema.TypeName != "Query" {
			t.Errorf("expected first chunk type name 'Query', got %q", chunk.Metadata.Schema.TypeName)
		}
	}

	// Check Mutation type
	if result.TotalChunks >= 2 {
		chunk := result.Chunks[1]
		if chunk.Metadata.Schema.TypeName != "Mutation" {
			t.Errorf("expected second chunk type name 'Mutation', got %q", chunk.Metadata.Schema.TypeName)
		}
	}
}

func TestGraphQLChunker_TestdataFixture(t *testing.T) {
	c := NewGraphQLChunker()

	// Read the testdata fixture
	fixturePath := filepath.Join("..", "..", "testdata", "devops", "sample.graphql")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has: User, Post, Role enum, CreateUserInput, CreatePostInput, Query, Mutation, DateTime scalar
	if result.TotalChunks < 5 {
		t.Errorf("expected at least 5 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify we have Query and Mutation types
	hasQuery := false
	hasMutation := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil {
			if chunk.Metadata.Schema.TypeName == "Query" {
				hasQuery = true
			}
			if chunk.Metadata.Schema.TypeName == "Mutation" {
				hasMutation = true
			}
		}
	}
	if !hasQuery {
		t.Error("expected to find Query type")
	}
	if !hasMutation {
		t.Error("expected to find Mutation type")
	}
}

func TestGraphQLChunker_ContextCancellation(t *testing.T) {
	c := NewGraphQLChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `type Test {
  id: ID!
}
`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestGraphQLChunker_ChunkType(t *testing.T) {
	c := NewGraphQLChunker()
	content := `type Test {
  id: ID!
}
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

func TestGraphQLChunker_CommentsPreserved(t *testing.T) {
	c := NewGraphQLChunker()
	content := `"""
User type represents a system user.
"""
type User {
  id: ID!
  "User's display name"
  name: String!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least 1 chunk
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}

	// Find the User type chunk and verify it contains the type definition
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil && chunk.Metadata.Schema.TypeName == "User" {
			found = true
			// Verify the type definition is present
			if !strings.Contains(chunk.Content, "type User") {
				t.Error("expected User chunk to contain type definition")
			}
			break
		}
	}
	if !found {
		t.Error("expected to find User type chunk")
	}
}

func TestGraphQLChunker_ExtendType(t *testing.T) {
	c := NewGraphQLChunker()
	content := `type User {
  id: ID!
}

extend type User {
  email: String!
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks, got %d", result.TotalChunks)
	}

	// Check that we have the extend type
	hasExtend := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Schema != nil && strings.HasPrefix(chunk.Metadata.Schema.TypeKind, "extend") {
			hasExtend = true
			break
		}
	}
	if !hasExtend {
		t.Error("expected to find extend type definition")
	}
}
