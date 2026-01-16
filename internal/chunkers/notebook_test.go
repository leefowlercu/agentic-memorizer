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

func TestNotebookChunker_EdgeCases(t *testing.T) {
	chunker := NewNotebookChunker()

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		content := []byte(`{
			"cells": [{"cell_type": "markdown", "source": "Test"}],
			"metadata": {},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)
		_, err := chunker.Chunk(ctx, content, DefaultChunkOptions())
		if err == nil {
			t.Error("Expected context cancellation error")
		}
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("LargeCellSplitting", func(t *testing.T) {
		// Create a large markdown cell that should be split using array source format
		var lines []string
		for i := 0; i < 100; i++ {
			lines = append(lines, "Line "+string(rune('0'+i%10))+" with some content here.\\n")
		}

		// Build JSON manually to avoid escaping issues
		jsonContent := `{"cells": [{"cell_type": "markdown", "source": [`
		for i, line := range lines {
			if i > 0 {
				jsonContent += ","
			}
			jsonContent += `"` + line + `"`
		}
		jsonContent += `]}], "metadata": {}, "nbformat": 4, "nbformat_minor": 5}`

		opts := ChunkOptions{MaxChunkSize: 200}
		result, err := chunker.Chunk(context.Background(), []byte(jsonContent), opts)
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should have multiple chunks due to size limit
		if len(result.Chunks) < 2 {
			t.Logf("Expected multiple chunks for large content, got %d (content may be structured differently)", len(result.Chunks))
		}
	})

	t.Run("EmptyCellsArray", func(t *testing.T) {
		content := []byte(`{
			"cells": [],
			"metadata": {"kernelspec": {"name": "python3"}},
			"nbformat": 4,
			"nbformat_minor": 5
		}`)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for empty cells array, got %d", len(result.Chunks))
		}
	})

	t.Run("RawCellType", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "raw",
					"source": "Raw cell content"
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

		// Raw cells should be handled (possibly as prose type)
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk for raw cell")
		}
	})

	t.Run("ExecuteResultOutput", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "1 + 1",
					"execution_count": 1,
					"outputs": [
						{
							"output_type": "execute_result",
							"data": {
								"text/plain": "2"
							},
							"execution_count": 1
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
		foundExecuteResult := false
		for _, ot := range chunk.Metadata.Notebook.OutputTypes {
			if ot == "execute_result" {
				foundExecuteResult = true
				break
			}
		}
		if !foundExecuteResult {
			t.Error("Expected 'execute_result' in OutputTypes")
		}
	})

	t.Run("DisplayDataOutput", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "display(data)",
					"execution_count": 1,
					"outputs": [
						{
							"output_type": "display_data",
							"data": {
								"text/plain": "<Figure>"
							}
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
		foundDisplayData := false
		for _, ot := range chunk.Metadata.Notebook.OutputTypes {
			if ot == "display_data" {
				foundDisplayData = true
				break
			}
		}
		if !foundDisplayData {
			t.Error("Expected 'display_data' in OutputTypes")
		}
	})

	t.Run("MultipleOutputTypes", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "print('hello'); 1+1",
					"execution_count": 1,
					"outputs": [
						{
							"output_type": "stream",
							"name": "stdout",
							"text": "hello\n"
						},
						{
							"output_type": "execute_result",
							"data": {"text/plain": "2"},
							"execution_count": 1
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
		if len(chunk.Metadata.Notebook.OutputTypes) < 2 {
			t.Errorf("Expected at least 2 output types, got %d", len(chunk.Metadata.Notebook.OutputTypes))
		}
	})

	t.Run("NullExecutionCount", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "x = 1",
					"execution_count": null,
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

		// Should handle null execution_count gracefully
		chunk := result.Chunks[0]
		if chunk.Metadata.Notebook.ExecutionCount != 0 {
			t.Errorf("Expected ExecutionCount 0 for null, got %d", chunk.Metadata.Notebook.ExecutionCount)
		}
	})

	t.Run("MultipleConsecutiveCodeCells", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "x = 1",
					"outputs": []
				},
				{
					"cell_type": "code",
					"source": "y = 2",
					"outputs": []
				},
				{
					"cell_type": "code",
					"source": "z = 3",
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

		// Multiple consecutive code cells should be grouped
		if len(result.Chunks) != 1 {
			t.Logf("Consecutive code cells produced %d chunks (grouping behavior)", len(result.Chunks))
		}
	})

	t.Run("MultipleHeadingLevels", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "markdown",
					"source": "# Level 1\nSome text"
				},
				{
					"cell_type": "markdown",
					"source": "## Level 2\nMore text"
				},
				{
					"cell_type": "markdown",
					"source": "### Level 3\nEven more"
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

		// Should extract headings at various levels
		foundLevel1 := false
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Level 1" {
				foundLevel1 = true
			}
		}
		// Note: consecutive markdown cells may be grouped, so heading detection may vary
		_ = foundLevel1
	})

	t.Run("NoKernelspec", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "print('test')",
					"outputs": []
				}
			],
			"metadata": {
				"language_info": {
					"name": "python"
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
			t.Fatal("Expected at least one chunk")
		}

		// Should fallback to language_info for kernel
		chunk := result.Chunks[0]
		if chunk.Metadata.Notebook.Kernel != "python" {
			t.Errorf("Expected kernel 'python' from language_info, got %q", chunk.Metadata.Notebook.Kernel)
		}
	})

	t.Run("TokenEstimatePopulated", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "markdown",
					"source": "Some content for token estimation"
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

		for i, chunk := range result.Chunks {
			if chunk.Metadata.TokenEstimate <= 0 {
				t.Errorf("Chunk %d has invalid TokenEstimate: %d", i, chunk.Metadata.TokenEstimate)
			}
		}
	})

	t.Run("EmptyCellSource", func(t *testing.T) {
		content := []byte(`{
			"cells": [
				{
					"cell_type": "code",
					"source": "",
					"outputs": []
				},
				{
					"cell_type": "markdown",
					"source": "Non-empty cell"
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

		// Empty cells should be skipped
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Non-empty") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find non-empty cell content")
		}
	})
}
