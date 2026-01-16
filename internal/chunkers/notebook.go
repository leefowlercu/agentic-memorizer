package chunkers

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

const (
	notebookChunkerName     = "notebook"
	notebookChunkerPriority = 76
)

// Regex for markdown headings
var notebookHeadingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// NotebookChunker splits Jupyter notebook content by cells with smart grouping.
type NotebookChunker struct{}

// NewNotebookChunker creates a new notebook chunker.
func NewNotebookChunker() *NotebookChunker {
	return &NotebookChunker{}
}

// Name returns the chunker's identifier.
func (c *NotebookChunker) Name() string {
	return notebookChunkerName
}

// CanHandle returns true for Jupyter notebook content.
func (c *NotebookChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "application/x-ipynb+json" ||
		mimeType == "application/json" && strings.HasSuffix(strings.ToLower(language), ".ipynb") ||
		strings.HasSuffix(strings.ToLower(language), ".ipynb")
}

// Priority returns the chunker's priority.
func (c *NotebookChunker) Priority() int {
	return notebookChunkerPriority
}

// Chunk splits notebook content by cells with smart grouping.
func (c *NotebookChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  notebookChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	// Parse notebook JSON
	var notebook jupyterNotebook
	if err := json.Unmarshal(content, &notebook); err != nil {
		return nil, err
	}

	// Extract kernel info
	kernel := c.extractKernel(notebook)

	// Process cells into groups
	groups := c.groupCells(notebook.Cells)

	var chunks []Chunk
	var warnings []ChunkWarning
	var offset int

	for _, group := range groups {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Build chunk content from grouped cells
		text, heading, outputTypes, hasOutput, execCount := c.buildGroupContent(group)

		// Skip empty groups
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// Determine cell type for the group
		cellType := group[0].CellType

		// If group is too large, split it
		if len(text) > maxSize {
			subChunks := c.splitLargeGroup(ctx, group, maxSize, kernel, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
			offset += len(text)
		} else {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     text,
				StartOffset: offset,
				EndOffset:   offset + len(text),
				Metadata: ChunkMetadata{
					Type:          c.getChunkType(cellType),
					TokenEstimate: EstimateTokens(text),
					Notebook: &NotebookMetadata{
						CellType:       cellType,
						CellIndex:      group[0].Index,
						ExecutionCount: execCount,
						HasOutput:      hasOutput,
						OutputTypes:    outputTypes,
						Kernel:         kernel,
					},
					Document: c.buildDocumentMetadata(heading),
				},
			})
			offset += len(text)
		}
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     warnings,
		TotalChunks:  len(chunks),
		ChunkerUsed:  notebookChunkerName,
		OriginalSize: len(content),
	}, nil
}

// Jupyter notebook JSON structures

type jupyterNotebook struct {
	Cells    []jupyterCell `json:"cells"`
	Metadata struct {
		Kernelspec struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Language    string `json:"language"`
		} `json:"kernelspec"`
		LanguageInfo struct {
			Name string `json:"name"`
		} `json:"language_info"`
	} `json:"metadata"`
	NBFormat      int `json:"nbformat"`
	NBFormatMinor int `json:"nbformat_minor"`
}

type jupyterCell struct {
	CellType       string          `json:"cell_type"`
	Source         json.RawMessage `json:"source"`
	Outputs        []jupyterOutput `json:"outputs,omitempty"`
	ExecutionCount *int            `json:"execution_count,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	Index          int             // Added during processing
}

type jupyterOutput struct {
	OutputType string          `json:"output_type"`
	Text       json.RawMessage `json:"text,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Name       string          `json:"name,omitempty"`
	EName      string          `json:"ename,omitempty"`
	EValue     string          `json:"evalue,omitempty"`
}

// cellGroup represents a group of consecutive same-type cells.
type cellGroup []jupyterCell

// extractKernel extracts the kernel name from notebook metadata.
func (c *NotebookChunker) extractKernel(notebook jupyterNotebook) string {
	if notebook.Metadata.Kernelspec.Name != "" {
		return notebook.Metadata.Kernelspec.Name
	}
	if notebook.Metadata.Kernelspec.DisplayName != "" {
		return notebook.Metadata.Kernelspec.DisplayName
	}
	if notebook.Metadata.LanguageInfo.Name != "" {
		return notebook.Metadata.LanguageInfo.Name
	}
	return ""
}

// groupCells groups consecutive cells of the same type.
func (c *NotebookChunker) groupCells(cells []jupyterCell) []cellGroup {
	if len(cells) == 0 {
		return nil
	}

	var groups []cellGroup
	var currentGroup cellGroup

	// Add index to each cell
	for i := range cells {
		cells[i].Index = i
	}

	for _, cell := range cells {
		if len(currentGroup) == 0 {
			currentGroup = append(currentGroup, cell)
		} else if currentGroup[0].CellType == cell.CellType {
			// Same type, add to current group
			currentGroup = append(currentGroup, cell)
		} else {
			// Different type, finalize current group and start new one
			groups = append(groups, currentGroup)
			currentGroup = cellGroup{cell}
		}
	}

	// Add final group
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// buildGroupContent builds the content string from a cell group.
func (c *NotebookChunker) buildGroupContent(group cellGroup) (text string, heading string, outputTypes []string, hasOutput bool, execCount int) {
	var builder strings.Builder
	outputTypeSet := make(map[string]bool)

	for _, cell := range group {
		// Extract source
		source := c.extractSource(cell.Source)

		// Add cell type marker for clarity
		if cell.CellType == "markdown" {
			// Check for heading
			if h, _ := c.extractHeading(source); h != "" && heading == "" {
				heading = h
			}
		} else if cell.CellType == "code" {
			builder.WriteString("```")
			builder.WriteString("\n")
		}

		builder.WriteString(source)
		if !strings.HasSuffix(source, "\n") {
			builder.WriteString("\n")
		}

		if cell.CellType == "code" {
			builder.WriteString("```")
			builder.WriteString("\n")

			// Process outputs
			for _, output := range cell.Outputs {
				hasOutput = true
				outputTypeSet[output.OutputType] = true

				// Add output content
				outputText := c.extractOutputText(output)
				if outputText != "" {
					builder.WriteString("\n# Output:\n")
					builder.WriteString(outputText)
					if !strings.HasSuffix(outputText, "\n") {
						builder.WriteString("\n")
					}
				}
			}

			// Track execution count
			if cell.ExecutionCount != nil {
				execCount = *cell.ExecutionCount
			}
		}

		builder.WriteString("\n")
	}

	// Convert output types to slice
	for ot := range outputTypeSet {
		outputTypes = append(outputTypes, ot)
	}

	return builder.String(), heading, outputTypes, hasOutput, execCount
}

// extractSource extracts the source string from the raw JSON.
func (c *NotebookChunker) extractSource(raw json.RawMessage) string {
	// Source can be a string or array of strings
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}

	return ""
}

// extractOutputText extracts text from an output.
func (c *NotebookChunker) extractOutputText(output jupyterOutput) string {
	switch output.OutputType {
	case "stream":
		return c.extractSource(output.Text)
	case "execute_result", "display_data":
		// Try to get text/plain from data
		var data map[string]json.RawMessage
		if err := json.Unmarshal(output.Data, &data); err == nil {
			if textPlain, ok := data["text/plain"]; ok {
				return c.extractSource(textPlain)
			}
		}
	case "error":
		if output.EName != "" {
			return output.EName + ": " + output.EValue
		}
	}
	return ""
}

// extractHeading extracts heading text and level from markdown content.
func (c *NotebookChunker) extractHeading(content string) (string, int) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		matches := notebookHeadingRegex.FindStringSubmatch(line)
		if matches != nil {
			level := len(matches[1])
			heading := strings.TrimSpace(matches[2])
			return heading, level
		}
	}
	return "", 0
}

// buildDocumentMetadata creates document metadata from heading info.
func (c *NotebookChunker) buildDocumentMetadata(heading string) *DocumentMetadata {
	if heading == "" {
		return nil
	}
	return &DocumentMetadata{
		Heading:      heading,
		HeadingLevel: 1, // Default to level 1 for notebook headings
	}
}

// getChunkType returns the appropriate chunk type for a cell type.
func (c *NotebookChunker) getChunkType(cellType string) ChunkType {
	switch cellType {
	case "code":
		return ChunkTypeCode
	case "markdown":
		return ChunkTypeMarkdown
	default:
		return ChunkTypeProse
	}
}

// splitLargeGroup splits a large cell group into smaller chunks.
func (c *NotebookChunker) splitLargeGroup(ctx context.Context, group cellGroup, maxSize int, kernel string, baseOffset int) []Chunk {
	var chunks []Chunk
	offset := baseOffset

	// Split each cell individually if the group is too large
	for _, cell := range group {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		source := c.extractSource(cell.Source)
		text := strings.TrimSpace(source)
		if text == "" {
			continue
		}

		// Extract heading for markdown cells
		var heading string
		if cell.CellType == "markdown" {
			heading, _ = c.extractHeading(source)
		}

		// Collect outputs
		var outputTypes []string
		hasOutput := false
		execCount := 0

		if cell.CellType == "code" {
			text = "```\n" + text + "\n```"

			for _, output := range cell.Outputs {
				hasOutput = true
				outputTypes = append(outputTypes, output.OutputType)

				outputText := c.extractOutputText(output)
				if outputText != "" {
					text += "\n# Output:\n" + outputText
				}
			}

			if cell.ExecutionCount != nil {
				execCount = *cell.ExecutionCount
			}
		}

		// If single cell is still too large, split by lines
		if len(text) > maxSize {
			subChunks := c.splitCellByLines(ctx, cell, text, maxSize, kernel, heading, outputTypes, hasOutput, execCount, offset)
			chunks = append(chunks, subChunks...)
			offset += len(text)
		} else {
			chunks = append(chunks, Chunk{
				Content:     text,
				StartOffset: offset,
				EndOffset:   offset + len(text),
				Metadata: ChunkMetadata{
					Type:          c.getChunkType(cell.CellType),
					TokenEstimate: EstimateTokens(text),
					Notebook: &NotebookMetadata{
						CellType:       cell.CellType,
						CellIndex:      cell.Index,
						ExecutionCount: execCount,
						HasOutput:      hasOutput,
						OutputTypes:    outputTypes,
						Kernel:         kernel,
					},
					Document: c.buildDocumentMetadata(heading),
				},
			})
			offset += len(text)
		}
	}

	return chunks
}

// splitCellByLines splits a single large cell by lines.
func (c *NotebookChunker) splitCellByLines(ctx context.Context, cell jupyterCell, text string, maxSize int, kernel, heading string, outputTypes []string, hasOutput bool, execCount int, baseOffset int) []Chunk {
	var chunks []Chunk
	lines := strings.Split(text, "\n")
	var current strings.Builder
	offset := baseOffset

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		// If adding this line exceeds max, finalize current chunk
		if current.Len()+len(line)+1 > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset,
				EndOffset:   offset + len(content),
				Metadata: ChunkMetadata{
					Type:          c.getChunkType(cell.CellType),
					TokenEstimate: EstimateTokens(content),
					Notebook: &NotebookMetadata{
						CellType:       cell.CellType,
						CellIndex:      cell.Index,
						ExecutionCount: execCount,
						HasOutput:      hasOutput,
						OutputTypes:    outputTypes,
						Kernel:         kernel,
					},
					Document: c.buildDocumentMetadata(heading),
				},
			})
			offset += len(content)
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset,
			EndOffset:   offset + len(content),
			Metadata: ChunkMetadata{
				Type:          c.getChunkType(cell.CellType),
				TokenEstimate: EstimateTokens(content),
				Notebook: &NotebookMetadata{
					CellType:       cell.CellType,
					CellIndex:      cell.Index,
					ExecutionCount: execCount,
					HasOutput:      hasOutput,
					OutputTypes:    outputTypes,
					Kernel:         kernel,
				},
				Document: c.buildDocumentMetadata(heading),
			},
		})
	}

	return chunks
}
