package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	makefileChunkerName     = "makefile"
	makefileChunkerPriority = 44
)

// Matches target definitions: target: [dependencies]
// Also matches pattern rules like %.o: %.c
var makefileTargetRegex = regexp.MustCompile(`^([a-zA-Z0-9_.%-]+(?:\s+[a-zA-Z0-9_.%-]+)*)\s*:\s*(.*)$`)

// Matches .PHONY declaration
var makefilePhonyRegex = regexp.MustCompile(`^\.PHONY\s*:\s*(.*)$`)

// Matches variable assignment: VAR = value, VAR := value, VAR ?= value, VAR += value
var makefileVarRegex = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)\s*[:?+]?=\s*(.*)$`)

// MakefileChunker splits Makefile content by targets.
type MakefileChunker struct{}

// NewMakefileChunker creates a new Makefile chunker.
func NewMakefileChunker() *MakefileChunker {
	return &MakefileChunker{}
}

// Name returns the chunker's identifier.
func (c *MakefileChunker) Name() string {
	return makefileChunkerName
}

// CanHandle returns true for Makefile content.
func (c *MakefileChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	mime := strings.ToLower(mimeType)

	// Match by MIME type
	if mime == "text/x-makefile" || mime == "application/x-makefile" {
		return true
	}

	// Match by filename pattern
	// Makefile, makefile, GNUmakefile, *.mk
	if lang == "makefile" ||
		lang == "gnumakefile" ||
		strings.HasSuffix(lang, "/makefile") ||
		strings.HasSuffix(lang, "/gnumakefile") ||
		strings.HasSuffix(lang, ".mk") {
		return true
	}

	// Check basename
	parts := strings.Split(lang, "/")
	if len(parts) > 0 {
		basename := parts[len(parts)-1]
		lower := strings.ToLower(basename)
		if lower == "makefile" || lower == "gnumakefile" {
			return true
		}
	}

	return false
}

// Priority returns the chunker's priority.
func (c *MakefileChunker) Priority() int {
	return makefileChunkerPriority
}

// Chunk splits Makefile content by targets.
func (c *MakefileChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  makefileChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	targets := c.splitByTargets(text)

	var chunks []Chunk
	offset := 0

	for _, target := range targets {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If target block is too large, split it further
		if len(target.content) > maxSize {
			subChunks := c.splitLargeTarget(ctx, target, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(target.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     target.content,
				StartOffset: offset,
				EndOffset:   offset + len(target.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(target.content),
					Build: &BuildMetadata{
						TargetName:   target.name,
						Dependencies: target.dependencies,
					},
				},
			})
		}

		offset += len(target.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  makefileChunkerName,
		OriginalSize: len(content),
	}, nil
}

// makefileTarget represents a target in a Makefile.
type makefileTarget struct {
	name         string
	dependencies []string
	content      string
}

// splitByTargets splits Makefile text into targets.
func (c *MakefileChunker) splitByTargets(text string) []makefileTarget {
	var targets []makefileTarget
	var currentContent strings.Builder
	var preambleContent strings.Builder
	var currentName string
	var currentDeps []string
	seenFirstTarget := false

	lines := strings.Split(text, "\n")
	phonyTargets := make(map[string]bool)

	// First pass: collect .PHONY declarations
	for _, line := range lines {
		if match := makefilePhonyRegex.FindStringSubmatch(line); match != nil {
			for _, target := range strings.Fields(match[1]) {
				phonyTargets[target] = true
			}
		}
	}

	flushTarget := func() {
		content := currentContent.String()

		if !seenFirstTarget {
			return
		}

		// Include preamble with first target
		if len(targets) == 0 && preambleContent.Len() > 0 {
			content = preambleContent.String() + content
		}

		if content != "" || currentName != "" {
			targets = append(targets, makefileTarget{
				name:         currentName,
				dependencies: currentDeps,
				content:      content,
			})
		}
		currentContent.Reset()
	}

	inRecipe := false
	for i, line := range lines {
		// Skip empty lines at start of file for preamble calculation
		trimmed := strings.TrimSpace(line)

		// Check if this is a recipe line (starts with tab)
		isRecipeLine := len(line) > 0 && line[0] == '\t'

		// Check for target definition (not a recipe line, not .PHONY, matches target pattern)
		if !isRecipeLine && !makefilePhonyRegex.MatchString(line) {
			if match := makefileTargetRegex.FindStringSubmatch(line); match != nil {
				// Check if the first "target" is actually a variable assignment pattern
				// (happens when VAR: value is interpreted as target)
				targetPart := strings.TrimSpace(match[1])
				if !makefileVarRegex.MatchString(line) && !strings.HasPrefix(trimmed, "#") {
					// This is a real target
					if seenFirstTarget {
						flushTarget()
					}
					seenFirstTarget = true
					inRecipe = true

					// Parse target names (could be multiple)
					targetNames := strings.Fields(targetPart)
					if len(targetNames) > 0 {
						currentName = targetNames[0] // Use first target as primary
					}

					// Parse dependencies
					depsStr := strings.TrimSpace(match[2])
					if depsStr != "" {
						currentDeps = strings.Fields(depsStr)
					} else {
						currentDeps = nil
					}
				}
			}
		}

		// Accumulate content
		if seenFirstTarget {
			currentContent.WriteString(line)
			if i < len(lines)-1 {
				currentContent.WriteString("\n")
			}
			// Track if we're still in a recipe block
			if !isRecipeLine && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				inRecipe = false
			}
			_ = inRecipe // Use the variable to prevent unused warning
		} else {
			// Accumulate preamble (variables, comments before first target)
			preambleContent.WriteString(line)
			preambleContent.WriteString("\n")
		}
	}

	flushTarget()

	// If no targets were found, return the whole content as one chunk
	if len(targets) == 0 && preambleContent.Len() > 0 {
		targets = append(targets, makefileTarget{
			name:         "",
			dependencies: nil,
			content:      preambleContent.String(),
		})
	}

	return targets
}

// splitLargeTarget splits a large target block into smaller chunks.
func (c *MakefileChunker) splitLargeTarget(ctx context.Context, target makefileTarget, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	offset := baseOffset

	lines := strings.Split(target.content, "\n")

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
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					Build: &BuildMetadata{
						TargetName:   target.name,
						Dependencies: target.dependencies,
					},
				},
			})
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
		offset += len(line) + 1
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				Build: &BuildMetadata{
					TargetName:   target.name,
					Dependencies: target.dependencies,
				},
			},
		})
	}

	return chunks
}
