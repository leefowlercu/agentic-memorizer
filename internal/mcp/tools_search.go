package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const (
	toolSearchMemory = "search_memory"

	defaultTopK           = 10
	maxTopK               = 50
	defaultSnippetMaxChar = 400
	maxSnippetMaxChar     = 2000

	minCandidateK = 100
	maxCandidateK = 500
)

type searchMemoryFilters struct {
	PathPrefix        string   `json:"path_prefix,omitempty"`
	IncludeExtensions []string `json:"include_extensions,omitempty"`
	ExcludeExtensions []string `json:"exclude_extensions,omitempty"`
	MinScore          *float64 `json:"min_score,omitempty"`
	IncludeSnippets   bool     `json:"include_snippets"`
	SnippetMaxChars   int      `json:"snippet_max_chars,omitempty"`
}

type searchMemoryHit struct {
	Rank             int     `json:"rank"`
	Score            float64 `json:"score"`
	FilePath         string  `json:"file_path"`
	FileURI          string  `json:"file_uri"`
	ChunkID          string  `json:"chunk_id"`
	ChunkIndex       int     `json:"chunk_index"`
	StartOffset      int     `json:"start_offset"`
	EndOffset        int     `json:"end_offset"`
	ChunkType        string  `json:"chunk_type"`
	Summary          string  `json:"summary,omitempty"`
	Provider         string  `json:"provider,omitempty"`
	Model            string  `json:"model,omitempty"`
	Snippet          string  `json:"snippet,omitempty"`
	SnippetTruncated bool    `json:"snippet_truncated,omitempty"`
	SnippetError     string  `json:"snippet_error,omitempty"`
}

type searchMemoryResult struct {
	Query    string              `json:"query"`
	TopK     int                 `json:"top_k"`
	Returned int                 `json:"returned"`
	Filters  searchMemoryFilters `json:"filters"`
	Hits     []searchMemoryHit   `json:"hits"`
}

func (s *Server) registerTools() {
	tool := mcp.NewTool(
		toolSearchMemory,
		mcp.WithTitleAnnotation("Semantic Search"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithDescription("Semantic search over remembered files using stored embeddings."),
		mcp.WithString(
			"query",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Natural language query to search for."),
		),
		mcp.WithNumber(
			"top_k",
			mcp.Min(1),
			mcp.Max(maxTopK),
			mcp.DefaultNumber(defaultTopK),
			mcp.Description("Maximum number of hits to return."),
		),
		mcp.WithNumber(
			"min_score",
			mcp.Min(0),
			mcp.Max(1),
			mcp.Description("Optional similarity threshold in [0.0, 1.0]."),
		),
		mcp.WithString(
			"path_prefix",
			mcp.Description("Optional absolute path prefix to constrain results."),
		),
		mcp.WithArray(
			"include_extensions",
			mcp.WithStringItems(),
			mcp.Description("Optional file extensions to include (e.g. .go, md)."),
		),
		mcp.WithArray(
			"exclude_extensions",
			mcp.WithStringItems(),
			mcp.Description("Optional file extensions to exclude."),
		),
		mcp.WithBoolean(
			"include_snippets",
			mcp.DefaultBool(false),
			mcp.Description("When true, include snippet text extracted from source files."),
		),
		mcp.WithNumber(
			"snippet_max_chars",
			mcp.Min(1),
			mcp.Max(maxSnippetMaxChar),
			mcp.DefaultNumber(defaultSnippetMaxChar),
			mcp.Description("Maximum characters per snippet when include_snippets=true."),
		),
		mcp.WithSchemaAdditionalProperties(false),
	)

	s.mcpServer.AddTool(tool, s.handleSearchMemory)
}

func (s *Server) handleSearchMemory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.graph == nil {
		return mcp.NewToolResultError("semantic search unavailable: graph is not configured"), nil
	}

	if s.embeddings == nil || !s.embeddings.Available() {
		return mcp.NewToolResultError("semantic search unavailable: embeddings provider is not available"), nil
	}

	query, err := request.RequireString("query")
	if err != nil || strings.TrimSpace(query) == "" {
		return mcp.NewToolResultError("query is required"), nil
	}
	query = strings.TrimSpace(query)

	topK := clampInt(request.GetInt("top_k", defaultTopK), 1, maxTopK)

	args := request.GetArguments()
	minScore, hasMinScore, err := parseOptionalMinScore(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pathPrefix := strings.TrimSpace(request.GetString("path_prefix", ""))
	if pathPrefix != "" {
		pathPrefix = filepath.Clean(pathPrefix)
		if !filepath.IsAbs(pathPrefix) {
			return mcp.NewToolResultError("path_prefix must be an absolute path"), nil
		}
		if !s.isPathRemembered(ctx, pathPrefix) {
			return mcp.NewToolResultError("path_prefix must be under a remembered path"), nil
		}
	}

	includeSet, err := normalizeExtensionSet(request.GetStringSlice("include_extensions", nil))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	excludeSet, err := normalizeExtensionSet(request.GetStringSlice("exclude_extensions", nil))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	includeSnippets := request.GetBool("include_snippets", false)
	snippetMaxChars := clampInt(request.GetInt("snippet_max_chars", defaultSnippetMaxChar), 1, maxSnippetMaxChar)

	embeddingResult, err := s.embeddings.Embed(ctx, providers.EmbeddingsRequest{
		Content: query,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to embed query: %v", err)), nil
	}

	candidateK := topK
	if hasMinScore || pathPrefix != "" || len(includeSet) > 0 || len(excludeSet) > 0 {
		candidateK = topK * 10
		if candidateK < minCandidateK {
			candidateK = minCandidateK
		}
		if candidateK > maxCandidateK {
			candidateK = maxCandidateK
		}
	}

	searchHits, err := s.graph.SearchSimilarChunks(ctx, embeddingResult.Embedding, candidateK)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("semantic search failed: %v", err)), nil
	}

	fileCache := map[string][]byte{}
	resultHits := make([]searchMemoryHit, 0, topK)
	for _, hit := range searchHits {
		chunk := hit.Chunk

		if !s.isPathRemembered(ctx, chunk.FilePath) {
			continue
		}
		if pathPrefix != "" && !hasPathPrefix(chunk.FilePath, pathPrefix) {
			continue
		}

		ext := strings.ToLower(filepath.Ext(chunk.FilePath))
		if len(includeSet) > 0 {
			if _, ok := includeSet[ext]; !ok {
				continue
			}
		}
		if _, excluded := excludeSet[ext]; excluded {
			continue
		}
		if hasMinScore && hit.Score < minScore {
			continue
		}

		out := searchMemoryHit{
			Score:       hit.Score,
			FilePath:    chunk.FilePath,
			FileURI:     ResourceURIFilePrefix + chunk.FilePath,
			ChunkID:     chunk.ID,
			ChunkIndex:  chunk.Index,
			StartOffset: chunk.StartOffset,
			EndOffset:   chunk.EndOffset,
			ChunkType:   chunk.ChunkType,
			Summary:     chunk.Summary,
			Provider:    hit.Provider,
			Model:       hit.Model,
		}

		if includeSnippets {
			snippet, truncated, snippetErr := extractSnippet(fileCache, chunk.FilePath, chunk.StartOffset, chunk.EndOffset, snippetMaxChars)
			if snippetErr != nil {
				out.SnippetError = snippetErr.Error()
			} else {
				out.Snippet = snippet
				out.SnippetTruncated = truncated
			}
		}

		resultHits = append(resultHits, out)
		if len(resultHits) >= topK {
			break
		}
	}

	for i := range resultHits {
		resultHits[i].Rank = i + 1
	}

	var minScorePtr *float64
	if hasMinScore {
		minScoreVal := minScore
		minScorePtr = &minScoreVal
	}

	response := searchMemoryResult{
		Query:    query,
		TopK:     topK,
		Returned: len(resultHits),
		Filters: searchMemoryFilters{
			PathPrefix:        pathPrefix,
			IncludeExtensions: sortedSetKeys(includeSet),
			ExcludeExtensions: sortedSetKeys(excludeSet),
			MinScore:          minScorePtr,
			IncludeSnippets:   includeSnippets,
			SnippetMaxChars:   snippetMaxChars,
		},
		Hits: resultHits,
	}

	return mcp.NewToolResultStructured(response, renderSearchMemoryText(response)), nil
}

func parseOptionalMinScore(args map[string]any) (float64, bool, error) {
	if args == nil {
		return 0, false, nil
	}

	raw, ok := args["min_score"]
	if !ok || raw == nil {
		return 0, false, nil
	}

	var score float64
	switch v := raw.(type) {
	case float64:
		score = v
	case int:
		score = float64(v)
	case int64:
		score = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false, fmt.Errorf("min_score must be a number between 0 and 1")
		}
		score = parsed
	default:
		return 0, false, fmt.Errorf("min_score must be a number between 0 and 1")
	}

	if score < 0 || score > 1 {
		return 0, false, fmt.Errorf("min_score must be between 0 and 1")
	}

	return score, true, nil
}

func normalizeExtensionSet(values []string) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(values))
	for _, raw := range values {
		ext := strings.ToLower(strings.TrimSpace(raw))
		if ext == "" {
			return nil, fmt.Errorf("extension values must not be empty")
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		set[ext] = struct{}{}
	}
	return set, nil
}

func sortedSetKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func hasPathPrefix(path, prefix string) bool {
	cleanPath := filepath.Clean(path)
	cleanPrefix := filepath.Clean(prefix)

	if cleanPath == cleanPrefix {
		return true
	}

	sep := string(os.PathSeparator)
	if cleanPrefix == sep {
		return strings.HasPrefix(cleanPath, sep)
	}

	return strings.HasPrefix(cleanPath, cleanPrefix+sep)
}

func extractSnippet(fileCache map[string][]byte, path string, start, end, maxChars int) (string, bool, error) {
	content, ok := fileCache[path]
	if !ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("failed to read file: %w", err)
		}
		content = data
		fileCache[path] = data
	}

	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(content) {
		return "", false, fmt.Errorf("chunk start offset out of range")
	}
	if end > len(content) {
		end = len(content)
	}
	if start == end {
		return "", false, fmt.Errorf("empty snippet range")
	}

	text := string(content[start:end])
	runes := []rune(text)
	if len(runes) > maxChars {
		return string(runes[:maxChars]), true, nil
	}

	return text, false, nil
}

func renderSearchMemoryText(result searchMemoryResult) string {
	if len(result.Hits) == 0 {
		return fmt.Sprintf("No semantic matches found for %q.", result.Query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Semantic matches for %q (%d result(s)):\n", result.Query, len(result.Hits))
	for _, hit := range result.Hits {
		fmt.Fprintf(
			&b,
			"%d. [%.4f] %s (chunk %d, bytes %d-%d)\n",
			hit.Rank,
			hit.Score,
			hit.FilePath,
			hit.ChunkIndex,
			hit.StartOffset,
			hit.EndOffset,
		)
		if hit.Summary != "" {
			fmt.Fprintf(&b, "   Summary: %s\n", compactWhitespace(hit.Summary))
		}
		if hit.Snippet != "" {
			fmt.Fprintf(&b, "   Snippet: %s\n", compactWhitespace(hit.Snippet))
		}
		if hit.SnippetError != "" {
			fmt.Fprintf(&b, "   Snippet error: %s\n", hit.SnippetError)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func compactWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
