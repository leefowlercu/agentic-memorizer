package chunkers

import (
	"context"
	"testing"
)

func TestNotebookChunker(t *testing.T) {
	chunker := NewNotebookChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "notebook" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "notebook")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 76 {
			t.Errorf("Priority() = %d, want 76", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"application/x-ipynb+json", "", true},
			{"", "file.ipynb", true},
			{"", "FILE.IPYNB", true},
			{"application/json", "notebook.ipynb", true},
			{"text/plain", "", false},
			{"application/json", "file.json", false},
		}

		for _, tt := range tests {
			result := chunker.CanHandle(tt.mimeType, tt.language)
			if result != tt.expected {
				t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, result, tt.expected)
			}
		}
	})

	t.Run("EmptyContent", func(t *testing.T) {
		result, err := chunker.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for empty content, got %d", len(result.Chunks))
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		content := []byte("this is not JSON")
		_, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err == nil {
			t.Error("Expected error for invalid JSON content")
		}
	})

	t.Run("BasicNotebook", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "markdown",
					"source": "# Introduction\n\nThis is a test notebook."
				},
				{
					"cell_type": "code",
					"source": "print('Hello')",
					"execution_count": 1,
					"outputs": []
				}
			],
			"metadata": {
				"kernelspec": {
					"name": "python3",
					"language": "python"
				}
			},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}

		// Verify kernel is extracted
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Notebook != nil && chunk.Metadata.Notebook.Kernel == "" {
				t.Error("Expected kernel to be set")
			}
		}
	})

	t.Run("CellGrouping", func(t *testing.T) {
		// Two consecutive markdown cells should be grouped
		content := []byte(`{
			"cells": [
				{
					"cell_type": "markdown",
					"source": "# Section 1"
				},
				{
					"cell_type": "markdown",
					"source": "More text in section 1"
				},
				{
					"cell_type": "code",
					"source": "x = 1",
					"outputs": []
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should have 2 chunks: grouped markdown and code
		if len(result.Chunks) != 2 {
			t.Errorf("Expected 2 chunks (grouped), got %d", len(result.Chunks))
		}
	})

	t.Run("OutputDetection", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "print('Hello')",
					"execution_count": 1,
					"outputs": [
						{
							"output_type": "stream",
							"name": "stdout",
							"text": "Hello\n"
						}
					]
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}

		chunk := result.Chunks[0]
		if chunk.Metadata.Notebook == nil {
			t.Fatal("Expected Notebook metadata")
		}

		if !chunk.Metadata.Notebook.HasOutput {
			t.Error("Expected HasOutput to be true")
		}

		foundStream := false
		for _, ot := range chunk.Metadata.Notebook.OutputTypes {
			if ot == "stream" {
				foundStream = true
				break
			}
		}
		if !foundStream {
			t.Error("Expected 'stream' in OutputTypes")
		}

		// Verify output content is included
		if !contains(chunk.Content, "Hello") {
			t.Error("Expected output content 'Hello' in chunk")
		}
	})

	t.Run("HeadingExtraction", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "markdown",
					"source": "# Main Title\n\nSome content here."
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}

		chunk := result.Chunks[0]
		if chunk.Metadata.Document == nil {
			t.Fatal("Expected Document metadata for heading")
		}

		if chunk.Metadata.Document.Heading != "Main Title" {
			t.Errorf("Heading = %q, want %q", chunk.Metadata.Document.Heading, "Main Title")
		}
	})

	t.Run("CellTypes", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "markdown",
					"source": "Text"
				},
				{
					"cell_type": "code",
					"source": "code",
					"outputs": []
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		for _, chunk := range result.Chunks {
			if chunk.Metadata.Notebook == nil {
				continue
			}
			switch chunk.Metadata.Notebook.CellType {
			case "markdown":
				if chunk.Metadata.Type != ChunkTypeMarkdown {
					t.Errorf("Markdown cell got type %v", chunk.Metadata.Type)
				}
			case "code":
				if chunk.Metadata.Type != ChunkTypeCode {
					t.Errorf("Code cell got type %v", chunk.Metadata.Type)
				}
			}
		}
	})

	t.Run("ExecutionCount", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "x = 1",
					"execution_count": 42,
					"outputs": []
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}

		chunk := result.Chunks[0]
		if chunk.Metadata.Notebook == nil {
			t.Fatal("Expected Notebook metadata")
		}

		if chunk.Metadata.Notebook.ExecutionCount != 42 {
			t.Errorf("ExecutionCount = %d, want 42", chunk.Metadata.Notebook.ExecutionCount)
		}
	})

	t.Run("ArraySource", func(t *testing.T) {
		// Notebook source can be array of strings
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": ["line1\n", "line2"],
					"outputs": []
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}

		// Content should include both lines
		if !contains(result.Chunks[0].Content, "line1") || !contains(result.Chunks[0].Content, "line2") {
			t.Error("Expected both lines in content")
		}
	})

	t.Run("ErrorOutput", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "1/0",
					"outputs": [
						{
							"output_type": "error",
							"ename": "ZeroDivisionError",
							"evalue": "division by zero"
						}
					]
				}
			],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}

		// Error should be in output types
		chunk := result.Chunks[0]
		foundError := false
		for _, ot := range chunk.Metadata.Notebook.OutputTypes {
			if ot == "error" {
				foundError = true
				break
			}
		}
		if !foundError {
			t.Error("Expected 'error' in OutputTypes")
		}

		// Error message should be in content
		if !contains(chunk.Content, "ZeroDivisionError") {
			t.Error("Expected error name in content")
		}
	})
}
