package analysis

import (
	"context"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

// mergeSemanticResults combines multiple chunk analysis results into one.
// It concatenates summaries and deduplicates tags, topics, entities, and references.
func mergeSemanticResults(ctx context.Context, provider providers.SemanticProvider, results []*SemanticResult) (*SemanticResult, error) {
	if len(results) == 0 {
		return &SemanticResult{}, nil
	}

	if len(results) == 1 {
		return results[0], nil
	}

	merged := &SemanticResult{
		Tags:       make([]string, 0),
		Topics:     make([]string, 0),
		Entities:   make([]Entity, 0),
		References: make([]Reference, 0),
		Keywords:   make([]string, 0),
	}

	// Collect all summaries
	var summaries []string

	// Track seen items for deduplication
	seenTags := make(map[string]bool)
	seenTopics := make(map[string]bool)
	seenEntities := make(map[string]bool)
	seenRefs := make(map[string]bool)
	seenKeywords := make(map[string]bool)

	// Collect max complexity
	maxComplexity := 0

	for _, result := range results {
		if result == nil {
			continue
		}

		// Collect summaries
		if result.Summary != "" {
			summaries = append(summaries, result.Summary)
		}

		// Dedupe and merge tags
		for _, tag := range result.Tags {
			key := strings.ToLower(tag)
			if !seenTags[key] {
				seenTags[key] = true
				merged.Tags = append(merged.Tags, tag)
			}
		}

		// Dedupe and merge topics
		for _, topic := range result.Topics {
			key := strings.ToLower(topic)
			if !seenTopics[key] {
				seenTopics[key] = true
				merged.Topics = append(merged.Topics, topic)
			}
		}

		// Dedupe and merge entities
		for _, entity := range result.Entities {
			key := strings.ToLower(entity.Name + "|" + entity.Type)
			if !seenEntities[key] {
				seenEntities[key] = true
				merged.Entities = append(merged.Entities, entity)
			}
		}

		// Dedupe and merge references
		for _, ref := range result.References {
			key := ref.Type + "|" + ref.Target
			if !seenRefs[key] {
				seenRefs[key] = true
				merged.References = append(merged.References, ref)
			}
		}

		// Dedupe and merge keywords
		for _, keyword := range result.Keywords {
			key := strings.ToLower(keyword)
			if !seenKeywords[key] {
				seenKeywords[key] = true
				merged.Keywords = append(merged.Keywords, keyword)
			}
		}

		// Track max complexity
		if result.Complexity > maxComplexity {
			maxComplexity = result.Complexity
		}
	}

	merged.Complexity = maxComplexity

	// Summarize the summaries if we have a provider
	if len(summaries) > 1 && provider != nil && provider.Available() {
		combinedSummary, err := summarizeSummaries(ctx, provider, summaries)
		if err == nil {
			merged.Summary = combinedSummary
		} else {
			// Fall back to concatenation
			merged.Summary = concatenateSummaries(summaries)
		}
	} else if len(summaries) > 0 {
		merged.Summary = concatenateSummaries(summaries)
	}

	return merged, nil
}

// summarizeSummaries uses the semantic provider to create a cohesive summary.
func summarizeSummaries(ctx context.Context, provider providers.SemanticProvider, summaries []string) (string, error) {
	// Combine all summaries into a single text block
	combined := "Combine these section summaries into a cohesive overall summary:\n\n"
	for i, s := range summaries {
		combined += "Section " + string(rune('1'+i)) + ": " + s + "\n\n"
	}

	req := providers.SemanticRequest{
		Content:  combined,
		MIMEType: "text/plain",
	}

	result, err := provider.Analyze(ctx, req)
	if err != nil {
		return "", err
	}

	return result.Summary, nil
}

// concatenateSummaries joins summaries with paragraph breaks.
func concatenateSummaries(summaries []string) string {
	if len(summaries) == 0 {
		return ""
	}

	if len(summaries) == 1 {
		return summaries[0]
	}

	// Join with paragraph breaks, trimming each summary
	var result strings.Builder
	for i, s := range summaries {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if i > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(s)
	}

	return result.String()
}

// MergeOptions configures the merge behavior.
type MergeOptions struct {
	// MaxTags limits the number of merged tags.
	MaxTags int

	// MaxTopics limits the number of merged topics.
	MaxTopics int

	// MaxEntities limits the number of merged entities.
	MaxEntities int

	// MaxKeywords limits the number of merged keywords.
	MaxKeywords int

	// SummarizeSummaries uses the provider to create a cohesive summary.
	SummarizeSummaries bool
}

// DefaultMergeOptions returns sensible defaults.
func DefaultMergeOptions() MergeOptions {
	return MergeOptions{
		MaxTags:            50,
		MaxTopics:          20,
		MaxEntities:        100,
		MaxKeywords:        50,
		SummarizeSummaries: true,
	}
}
