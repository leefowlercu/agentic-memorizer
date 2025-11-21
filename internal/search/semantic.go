package search

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// SearchQuery represents a search request
type SearchQuery struct {
	Query      string   // Search term
	Categories []string // Filter by categories (empty = all)
	MaxResults int      // Maximum results to return (0 = unlimited)
}

// SearchResult represents a single search result
type SearchResult struct {
	Entry     types.IndexEntry // The matched index entry
	Score     float64          // Relevance score
	MatchType string           // Type of match (filename, summary, tag, topic, document_type)
}

// Searcher performs semantic search over an index
type Searcher struct {
	index *types.Index
}

// NewSearcher creates a new searcher for the given index
func NewSearcher(index *types.Index) *Searcher {
	return &Searcher{index: index}
}

// Common stop words to filter out from search queries
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
	"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
	"that": true, "the": true, "to": true, "was": true, "will": true, "with": true,
}

// tokenizeQuery splits query into searchable tokens
func tokenizeQuery(query string) []string {
	// Convert to lowercase and split on whitespace
	words := strings.Fields(strings.ToLower(query))

	// Filter out stop words and short tokens
	var tokens []string
	for _, word := range words {
		// Remove common punctuation
		word = strings.Trim(word, ".,!?;:\"'()[]{}")

		// Skip stop words and very short tokens
		if len(word) > 1 && !stopWords[word] {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

// Search performs a semantic search and returns ranked results
func (s *Searcher) Search(query SearchQuery) []SearchResult {
	if query.Query == "" {
		return []SearchResult{}
	}

	tokens := tokenizeQuery(query.Query)
	if len(tokens) == 0 {
		return []SearchResult{}
	}

	var results []SearchResult

	// Create category filter map for efficient lookup
	categoryFilter := make(map[string]bool)
	if len(query.Categories) > 0 {
		for _, cat := range query.Categories {
			categoryFilter[strings.ToLower(cat)] = true
		}
	}

	// Score each entry
	for _, entry := range s.index.Entries {
		// Apply category filter
		if len(categoryFilter) > 0 {
			if !categoryFilter[strings.ToLower(entry.Metadata.Category)] {
				continue
			}
		}

		score, matchType := s.scoreEntry(entry, tokens)
		if score > 0.1 { // Minimum score threshold for relevance
			results = append(results, SearchResult{
				Entry:     entry,
				Score:     score,
				MatchType: matchType,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results if specified
	if query.MaxResults > 0 && len(results) > query.MaxResults {
		results = results[:query.MaxResults]
	}

	return results
}

// scoreEntry calculates relevance score for an entry using token-based matching
func (s *Searcher) scoreEntry(entry types.IndexEntry, tokens []string) (float64, string) {
	var score float64
	var matchType string
	totalTokens := float64(len(tokens))

	// Helper function to count token matches in a text field
	countMatches := func(text string) int {
		textLower := strings.ToLower(text)
		matches := 0
		for _, token := range tokens {
			if strings.Contains(textLower, token) {
				matches++
			}
		}
		return matches
	}

	// 1. Filename match (highest weight: 3.0)
	filename := strings.ToLower(filepath.Base(entry.Metadata.Path))
	filenameMatches := countMatches(filename)
	if filenameMatches > 0 {
		score += (float64(filenameMatches) / totalTokens) * 3.0
		matchType = "filename"
	}

	// 2. Category match (1.0) - NEW
	categoryMatches := countMatches(entry.Metadata.Category)
	if categoryMatches > 0 {
		score += (float64(categoryMatches) / totalTokens) * 1.0
		if matchType == "" {
			matchType = "category"
		}
	}

	// 3. File type match (0.5) - NEW
	// Check both the Type field and extract extension from filename
	fileTypeMatches := countMatches(entry.Metadata.Type)
	ext := strings.TrimPrefix(filepath.Ext(entry.Metadata.Path), ".")
	if ext != "" {
		fileTypeMatches += countMatches(ext)
	}
	if fileTypeMatches > 0 {
		score += (float64(fileTypeMatches) / totalTokens) * 0.5
		if matchType == "" {
			matchType = "file_type"
		}
	}

	// If no semantic analysis, return metadata-only score
	if entry.Semantic == nil {
		return score, matchType
	}

	// 4. Summary match (2.0)
	summaryMatches := countMatches(entry.Semantic.Summary)
	if summaryMatches > 0 {
		score += (float64(summaryMatches) / totalTokens) * 2.0
		if matchType == "" {
			matchType = "summary"
		}
	}

	// 5. Tag matches (1.5)
	tagMatches := 0
	for _, tag := range entry.Semantic.Tags {
		tagMatches += countMatches(tag)
	}
	if tagMatches > 0 {
		score += (float64(tagMatches) / totalTokens) * 1.5
		if matchType == "" {
			matchType = "tag"
		}
	}

	// 6. Topic matches (1.0)
	topicMatches := 0
	for _, topic := range entry.Semantic.KeyTopics {
		topicMatches += countMatches(topic)
	}
	if topicMatches > 0 {
		score += (float64(topicMatches) / totalTokens) * 1.0
		if matchType == "" {
			matchType = "topic"
		}
	}

	// 7. Document type match (0.5)
	docTypeMatches := countMatches(entry.Semantic.DocumentType)
	if docTypeMatches > 0 {
		score += (float64(docTypeMatches) / totalTokens) * 0.5
		if matchType == "" {
			matchType = "document_type"
		}
	}

	return score, matchType
}
