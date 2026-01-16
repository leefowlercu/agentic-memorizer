package chunkers

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

var (
	tokenizer     *tiktoken.Tiktoken
	tokenizerOnce sync.Once
	tokenizerErr  error
)

// getTokenizer returns a cached tiktoken encoder.
// Uses cl100k_base encoding (GPT-4, GPT-3.5-turbo, text-embedding-ada-002).
func getTokenizer() (*tiktoken.Tiktoken, error) {
	tokenizerOnce.Do(func() {
		tokenizer, tokenizerErr = tiktoken.GetEncoding("cl100k_base")
	})
	return tokenizer, tokenizerErr
}

// CountTokens returns the accurate token count for text using tiktoken.
// Falls back to heuristic (~4 chars per token) if tiktoken fails.
func CountTokens(text string) int {
	enc, err := getTokenizer()
	if err != nil {
		// Fallback to heuristic if tiktoken fails to initialize
		return (len(text) + 3) / 4
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens)
}

// EstimateTokens returns an accurate token count for content using tiktoken.
// This is the primary function used by chunkers.
func EstimateTokens(text string) int {
	return CountTokens(text)
}

// EstimateTokensBytes returns an accurate token count for byte content.
func EstimateTokensBytes(content []byte) int {
	return CountTokens(string(content))
}
