package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerfileChunker_Name(t *testing.T) {
	c := NewDockerfileChunker()
	if c.Name() != "dockerfile" {
		t.Errorf("expected name 'dockerfile', got %q", c.Name())
	}
}

func TestDockerfileChunker_Priority(t *testing.T) {
	c := NewDockerfileChunker()
	if c.Priority() != 45 {
		t.Errorf("expected priority 45, got %d", c.Priority())
	}
}

func TestDockerfileChunker_CanHandle(t *testing.T) {
	c := NewDockerfileChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-dockerfile", "", true},
		{"application/x-dockerfile", "", true},
		{"", "dockerfile", true},
		{"", "Dockerfile", true},
		{"", "/path/to/Dockerfile", true},
		{"", "app.dockerfile", true},
		{"", "Dockerfile.prod", true},
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

func TestDockerfileChunker_EmptyContent(t *testing.T) {
	c := NewDockerfileChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "dockerfile" {
		t.Errorf("expected chunker name 'dockerfile', got %q", result.ChunkerUsed)
	}
}

func TestDockerfileChunker_SingleStage(t *testing.T) {
	c := NewDockerfileChunker()
	content := `FROM alpine:latest

RUN apk add --no-cache curl

CMD ["curl", "--version"]
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
		if chunk.Metadata.Build == nil {
			t.Fatal("expected Build metadata to be set")
		}
		if chunk.Metadata.Build.BaseImage != "alpine:latest" {
			t.Errorf("expected base image 'alpine:latest', got %q", chunk.Metadata.Build.BaseImage)
		}
		// Stage name should be "stage1" since no AS clause
		if chunk.Metadata.Build.StageName != "stage1" {
			t.Errorf("expected stage name 'stage1', got %q", chunk.Metadata.Build.StageName)
		}
	}
}

func TestDockerfileChunker_MultiStage(t *testing.T) {
	c := NewDockerfileChunker()
	content := `FROM golang:1.22 AS builder

WORKDIR /app
COPY . .
RUN go build -o /app/bin/server

FROM alpine:latest

COPY --from=builder /app/bin/server /server
CMD ["/server"]
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks (one per stage), got %d", result.TotalChunks)
	}

	// First stage should be named "builder"
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build == nil {
			t.Fatal("expected Build metadata for stage 1")
		}
		if chunk.Metadata.Build.StageName != "builder" {
			t.Errorf("expected stage name 'builder', got %q", chunk.Metadata.Build.StageName)
		}
		if chunk.Metadata.Build.BaseImage != "golang:1.22" {
			t.Errorf("expected base image 'golang:1.22', got %q", chunk.Metadata.Build.BaseImage)
		}
	}

	// Second stage should have auto-generated name
	if result.TotalChunks >= 2 {
		chunk := result.Chunks[1]
		if chunk.Metadata.Build == nil {
			t.Fatal("expected Build metadata for stage 2")
		}
		if chunk.Metadata.Build.StageName != "stage2" {
			t.Errorf("expected stage name 'stage2', got %q", chunk.Metadata.Build.StageName)
		}
		if chunk.Metadata.Build.BaseImage != "alpine:latest" {
			t.Errorf("expected base image 'alpine:latest', got %q", chunk.Metadata.Build.BaseImage)
		}
	}
}

func TestDockerfileChunker_FromWithPlatform(t *testing.T) {
	c := NewDockerfileChunker()
	content := `FROM --platform=linux/amd64 golang:1.22 AS builder

RUN go build
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
		if chunk.Metadata.Build == nil {
			t.Fatal("expected Build metadata")
		}
		if chunk.Metadata.Build.BaseImage != "golang:1.22" {
			t.Errorf("expected base image 'golang:1.22', got %q", chunk.Metadata.Build.BaseImage)
		}
		if chunk.Metadata.Build.StageName != "builder" {
			t.Errorf("expected stage name 'builder', got %q", chunk.Metadata.Build.StageName)
		}
	}
}

func TestDockerfileChunker_TestdataFixture(t *testing.T) {
	c := NewDockerfileChunker()

	// Read the testdata fixture
	fixturePath := filepath.Join("..", "..", "testdata", "devops", "Dockerfile")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has 2 stages: builder and final
	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify first stage is builder
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.StageName != "builder" {
			t.Errorf("expected first stage name 'builder', got %q", chunk.Metadata.Build.StageName)
		}
		if chunk.Metadata.Build.BaseImage != "golang:1.22-alpine" {
			t.Errorf("expected first stage base image 'golang:1.22-alpine', got %q", chunk.Metadata.Build.BaseImage)
		}
	}

	// Verify second stage
	if result.TotalChunks >= 2 {
		chunk := result.Chunks[1]
		if chunk.Metadata.Build.StageName != "stage2" {
			t.Errorf("expected second stage name 'stage2', got %q", chunk.Metadata.Build.StageName)
		}
		if chunk.Metadata.Build.BaseImage != "alpine:latest" {
			t.Errorf("expected second stage base image 'alpine:latest', got %q", chunk.Metadata.Build.BaseImage)
		}
	}
}

func TestDockerfileChunker_ContextCancellation(t *testing.T) {
	c := NewDockerfileChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `FROM alpine
RUN echo "test"
`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestDockerfileChunker_LineContinuation(t *testing.T) {
	c := NewDockerfileChunker()
	content := `FROM alpine

RUN apk add --no-cache \
    curl \
    wget \
    git

CMD ["sh"]
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	// Ensure the RUN command with continuation is kept together
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "apk add") {
			t.Error("expected chunk to contain RUN apk add")
		}
		if !strings.Contains(chunk.Content, "curl") {
			t.Error("expected chunk to contain curl (continuation)")
		}
	}
}

func TestDockerfileChunker_ChunkType(t *testing.T) {
	c := NewDockerfileChunker()
	content := `FROM alpine
RUN echo "test"
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

func TestDockerfileChunker_CommentsHandling(t *testing.T) {
	c := NewDockerfileChunker()
	content := `# This is a comment
FROM alpine

# Install packages
RUN apk add curl
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	// Comments should be included in the stage
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "# Install packages") {
			t.Error("expected chunk to contain comment")
		}
	}
}
