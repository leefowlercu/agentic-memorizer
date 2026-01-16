package chunkers

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

const (
	dockerfileChunkerName     = "dockerfile"
	dockerfileChunkerPriority = 45
)

// Matches FROM instruction with optional AS clause.
// FROM image:tag AS stage_name
// FROM image:tag
// FROM --platform=linux/amd64 image:tag AS stage_name
var dockerfileFromRegex = regexp.MustCompile(`(?i)^\s*FROM\s+(?:--\S+\s+)*([^\s]+)(?:\s+AS\s+(\S+))?`)

// DockerfileChunker splits Dockerfile content by build stages.
type DockerfileChunker struct{}

// NewDockerfileChunker creates a new Dockerfile chunker.
func NewDockerfileChunker() *DockerfileChunker {
	return &DockerfileChunker{}
}

// Name returns the chunker's identifier.
func (c *DockerfileChunker) Name() string {
	return dockerfileChunkerName
}

// CanHandle returns true for Dockerfile content.
func (c *DockerfileChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	mime := strings.ToLower(mimeType)

	// Match by MIME type
	if mime == "text/x-dockerfile" || mime == "application/x-dockerfile" {
		return true
	}

	// Match by filename pattern
	// - "dockerfile" (exact match)
	// - "/path/to/Dockerfile" (ends with dockerfile)
	// - "app.dockerfile" (ends with .dockerfile)
	// - "Dockerfile.prod" (contains dockerfile. pattern)
	if lang == "dockerfile" ||
		strings.HasSuffix(lang, "/dockerfile") ||
		strings.HasSuffix(lang, ".dockerfile") ||
		strings.Contains(lang, "dockerfile.") {
		return true
	}

	// Also match exact "dockerfile" as a filename component
	parts := strings.Split(lang, "/")
	if len(parts) > 0 {
		basename := parts[len(parts)-1]
		if strings.HasPrefix(basename, "dockerfile") || strings.HasSuffix(basename, "dockerfile") {
			return true
		}
	}

	return false
}

// Priority returns the chunker's priority.
func (c *DockerfileChunker) Priority() int {
	return dockerfileChunkerPriority
}

// Chunk splits Dockerfile content by FROM instructions (build stages).
func (c *DockerfileChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  dockerfileChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	stages := c.splitByStages(text)

	var chunks []Chunk
	offset := 0

	for _, stage := range stages {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If stage is too large, split it further
		if len(stage.content) > maxSize {
			subChunks := c.splitLargeStage(ctx, stage, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(stage.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     stage.content,
				StartOffset: offset,
				EndOffset:   offset + len(stage.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(stage.content),
					Build: &BuildMetadata{
						StageName: stage.stageName,
						BaseImage: stage.baseImage,
					},
				},
			})
		}

		offset += len(stage.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  dockerfileChunkerName,
		OriginalSize: len(content),
	}, nil
}

// dockerfileStage represents a build stage in a Dockerfile.
type dockerfileStage struct {
	stageName string
	baseImage string
	content   string
}

// splitByStages splits Dockerfile text into stages based on FROM instructions.
func (c *DockerfileChunker) splitByStages(text string) []dockerfileStage {
	var stages []dockerfileStage
	var currentContent strings.Builder
	var preambleContent strings.Builder // Content before first FROM
	var currentStageName string
	var currentBaseImage string
	stageCounter := 0
	seenFirstFrom := false

	lines := strings.Split(text, "\n")

	flushStage := func() {
		content := currentContent.String()

		// Prepend preamble to first stage
		if !seenFirstFrom {
			// This shouldn't happen - we only flush after seeing FROM
			return
		}

		if stageCounter == 1 && preambleContent.Len() > 0 {
			// Include preamble with the first stage
			content = preambleContent.String() + content
		}

		if content != "" || currentBaseImage != "" {
			// Generate stage name if not specified
			stageName := currentStageName
			if stageName == "" && currentBaseImage != "" {
				stageName = "stage" + strconv.Itoa(stageCounter)
			}

			stages = append(stages, dockerfileStage{
				stageName: stageName,
				baseImage: currentBaseImage,
				content:   content,
			})
		}
		currentContent.Reset()
	}

	for i, line := range lines {
		// Check for FROM instruction
		if match := dockerfileFromRegex.FindStringSubmatch(line); match != nil {
			// Flush previous stage (only if we've seen a FROM before)
			if seenFirstFrom {
				flushStage()
			}

			seenFirstFrom = true

			// Start new stage
			currentBaseImage = match[1]
			if len(match) > 2 && match[2] != "" {
				currentStageName = match[2]
			} else {
				currentStageName = ""
			}
			stageCounter++
		}

		// Accumulate content
		if seenFirstFrom {
			currentContent.WriteString(line)
			if i < len(lines)-1 {
				currentContent.WriteString("\n")
			}
		} else {
			// Accumulate preamble (before first FROM)
			preambleContent.WriteString(line)
			preambleContent.WriteString("\n")
		}
	}

	flushStage()
	return stages
}

// splitLargeStage splits a large stage into smaller chunks.
func (c *DockerfileChunker) splitLargeStage(ctx context.Context, stage dockerfileStage, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk

	// Split by instruction groups (RUN, COPY, etc.)
	groups := c.splitByInstructions(stage.content)
	var current strings.Builder
	offset := baseOffset

	for _, group := range groups {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}

		// If adding this group exceeds max, finalize current chunk
		if current.Len()+len(group)+1 > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					Build: &BuildMetadata{
						StageName: stage.stageName,
						BaseImage: stage.baseImage,
					},
				},
			})
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(group)
		offset += len(group) + 1
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
					StageName: stage.stageName,
					BaseImage: stage.baseImage,
				},
			},
		})
	}

	return chunks
}

// splitByInstructions splits content by Dockerfile instruction boundaries.
func (c *DockerfileChunker) splitByInstructions(content string) []string {
	var groups []string
	var current strings.Builder

	lines := strings.Split(content, "\n")
	inContinuation := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line is a continuation
		if inContinuation {
			current.WriteString("\n")
			current.WriteString(line)
			// Check if continuation continues
			inContinuation = strings.HasSuffix(trimmed, "\\")
			continue
		}

		// Check if this is a comment or empty line - include with previous group
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			current.WriteString("\n")
			current.WriteString(line)
			continue
		}

		// Check if this starts a new instruction (uppercase word followed by space or newline)
		isInstruction := isDockerInstruction(trimmed)
		if isInstruction && current.Len() > 0 {
			groups = append(groups, current.String())
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)

		// Check for line continuation
		inContinuation = strings.HasSuffix(trimmed, "\\")
	}

	if current.Len() > 0 {
		groups = append(groups, current.String())
	}

	return groups
}

// isDockerInstruction checks if a line starts with a Dockerfile instruction.
func isDockerInstruction(line string) bool {
	instructions := []string{
		"FROM", "RUN", "CMD", "LABEL", "MAINTAINER", "EXPOSE", "ENV",
		"ADD", "COPY", "ENTRYPOINT", "VOLUME", "USER", "WORKDIR",
		"ARG", "ONBUILD", "STOPSIGNAL", "HEALTHCHECK", "SHELL",
	}

	upper := strings.ToUpper(line)
	for _, inst := range instructions {
		if strings.HasPrefix(upper, inst+" ") || strings.HasPrefix(upper, inst+"\t") || upper == inst {
			return true
		}
	}
	return false
}
