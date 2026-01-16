package graph

import (
	"strings"
	"testing"
)

func TestCoreIndexesDefinitions(t *testing.T) {
	t.Run("core indexes are not empty", func(t *testing.T) {
		if len(coreIndexes) == 0 {
			t.Error("coreIndexes should not be empty")
		}
	})

	t.Run("core indexes contain expected labels", func(t *testing.T) {
		expectedLabels := []string{"File", "Chunk", "Directory", "Tag", "Topic", "Entity"}
		for _, label := range expectedLabels {
			found := false
			for _, idx := range coreIndexes {
				if strings.Contains(idx, label) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected core index for label %q not found", label)
			}
		}
	})

	t.Run("file path index exists", func(t *testing.T) {
		found := false
		for _, idx := range coreIndexes {
			if strings.Contains(idx, "File") && strings.Contains(idx, "path") {
				found = true
				break
			}
		}
		if !found {
			t.Error("File path index not found")
		}
	})

	t.Run("chunk id index exists", func(t *testing.T) {
		found := false
		for _, idx := range coreIndexes {
			if strings.Contains(idx, "Chunk") && strings.Contains(idx, "id") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Chunk id index not found")
		}
	})

	t.Run("content_hash indexes exist", func(t *testing.T) {
		foundFile := false
		foundChunk := false
		for _, idx := range coreIndexes {
			if strings.Contains(idx, "content_hash") {
				if strings.Contains(idx, "File") {
					foundFile = true
				}
				if strings.Contains(idx, "Chunk") {
					foundChunk = true
				}
			}
		}
		if !foundFile {
			t.Error("File content_hash index not found")
		}
		if !foundChunk {
			t.Error("Chunk content_hash index not found")
		}
	})

	t.Run("normalized_name indexes exist", func(t *testing.T) {
		expectedLabels := []string{"Tag", "Topic", "Entity"}
		for _, label := range expectedLabels {
			found := false
			for _, idx := range coreIndexes {
				if strings.Contains(idx, label) && strings.Contains(idx, "normalized_name") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("normalized_name index for %q not found", label)
			}
		}
	})

	t.Run("all indexes are CREATE INDEX statements", func(t *testing.T) {
		for _, idx := range coreIndexes {
			if !strings.HasPrefix(idx, "CREATE INDEX") {
				t.Errorf("Index does not start with CREATE INDEX: %q", idx)
			}
		}
	})
}

func TestMetadataIndexesDefinitions(t *testing.T) {
	t.Run("metadata indexes are not empty", func(t *testing.T) {
		if len(metadataIndexes) == 0 {
			t.Error("metadataIndexes should not be empty")
		}
	})

	t.Run("CodeMeta indexes exist", func(t *testing.T) {
		expectedFields := []string{"function_name", "class_name", "language", "visibility"}
		for _, field := range expectedFields {
			found := false
			for _, idx := range metadataIndexes {
				if strings.Contains(idx, "CodeMeta") && strings.Contains(idx, field) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("CodeMeta index for %q not found", field)
			}
		}
	})

	t.Run("DocumentMeta indexes exist", func(t *testing.T) {
		expectedFields := []string{"heading", "heading_level"}
		for _, field := range expectedFields {
			found := false
			for _, idx := range metadataIndexes {
				if strings.Contains(idx, "DocumentMeta") && strings.Contains(idx, field) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("DocumentMeta index for %q not found", field)
			}
		}
	})

	t.Run("NotebookMeta cell_type index exists", func(t *testing.T) {
		found := false
		for _, idx := range metadataIndexes {
			if strings.Contains(idx, "NotebookMeta") && strings.Contains(idx, "cell_type") {
				found = true
				break
			}
		}
		if !found {
			t.Error("NotebookMeta cell_type index not found")
		}
	})

	t.Run("BuildMeta indexes exist", func(t *testing.T) {
		expectedFields := []string{"target_name", "stage_name"}
		for _, field := range expectedFields {
			found := false
			for _, idx := range metadataIndexes {
				if strings.Contains(idx, "BuildMeta") && strings.Contains(idx, field) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("BuildMeta index for %q not found", field)
			}
		}
	})

	t.Run("InfraMeta indexes exist", func(t *testing.T) {
		expectedFields := []string{"resource_type", "block_type"}
		for _, field := range expectedFields {
			found := false
			for _, idx := range metadataIndexes {
				if strings.Contains(idx, "InfraMeta") && strings.Contains(idx, field) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("InfraMeta index for %q not found", field)
			}
		}
	})

	t.Run("SchemaMeta indexes exist", func(t *testing.T) {
		expectedFields := []string{"message_name", "type_name"}
		for _, field := range expectedFields {
			found := false
			for _, idx := range metadataIndexes {
				if strings.Contains(idx, "SchemaMeta") && strings.Contains(idx, field) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("SchemaMeta index for %q not found", field)
			}
		}
	})

	t.Run("ChunkEmbedding indexes exist", func(t *testing.T) {
		expectedFields := []string{"provider", "model"}
		for _, field := range expectedFields {
			found := false
			for _, idx := range metadataIndexes {
				if strings.Contains(idx, "ChunkEmbedding") && strings.Contains(idx, field) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ChunkEmbedding index for %q not found", field)
			}
		}
	})

	t.Run("all metadata indexes are CREATE INDEX statements", func(t *testing.T) {
		for _, idx := range metadataIndexes {
			if !strings.HasPrefix(idx, "CREATE INDEX") {
				t.Errorf("Index does not start with CREATE INDEX: %q", idx)
			}
		}
	})
}

func TestIndexSyntax(t *testing.T) {
	allIndexes := append(coreIndexes, metadataIndexes...)

	t.Run("all indexes have FOR clause", func(t *testing.T) {
		for _, idx := range allIndexes {
			if !strings.Contains(idx, "FOR") {
				t.Errorf("Index missing FOR clause: %q", idx)
			}
		}
	})

	t.Run("all indexes have ON clause", func(t *testing.T) {
		for _, idx := range allIndexes {
			if !strings.Contains(idx, "ON") {
				t.Errorf("Index missing ON clause: %q", idx)
			}
		}
	})

	t.Run("indexes use valid property syntax", func(t *testing.T) {
		for _, idx := range allIndexes {
			// Should have pattern like (n.property)
			if !strings.Contains(idx, "(") || !strings.Contains(idx, ".") || !strings.Contains(idx, ")") {
				t.Errorf("Index has invalid property syntax: %q", idx)
			}
		}
	})
}

func TestTotalIndexCount(t *testing.T) {
	t.Run("reasonable number of core indexes", func(t *testing.T) {
		// Should have indexes for primary lookups
		if len(coreIndexes) < 5 {
			t.Errorf("Expected at least 5 core indexes, got %d", len(coreIndexes))
		}
	})

	t.Run("reasonable number of metadata indexes", func(t *testing.T) {
		// Should have indexes for metadata type lookups
		if len(metadataIndexes) < 10 {
			t.Errorf("Expected at least 10 metadata indexes, got %d", len(metadataIndexes))
		}
	})
}
