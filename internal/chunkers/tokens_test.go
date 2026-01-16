package chunkers

import (
	"strings"
	"sync"
	"testing"
)

func TestCountTokensBasic(t *testing.T) {
	t.Run("empty string returns zero", func(t *testing.T) {
		result := CountTokens("")
		if result != 0 {
			t.Errorf("CountTokens(\"\") = %d, want 0", result)
		}
	})

	t.Run("single word returns positive count", func(t *testing.T) {
		result := CountTokens("hello")
		if result < 1 {
			t.Errorf("CountTokens(\"hello\") = %d, want >= 1", result)
		}
	})

	t.Run("short phrase returns positive count", func(t *testing.T) {
		result := CountTokens("hello world")
		if result < 2 {
			t.Errorf("CountTokens(\"hello world\") = %d, want >= 2", result)
		}
	})

	t.Run("whitespace is tokenized", func(t *testing.T) {
		result := CountTokens("     ")
		if result < 1 {
			t.Errorf("CountTokens with spaces = %d, want >= 1", result)
		}
	})

	t.Run("longer text has more tokens", func(t *testing.T) {
		short := CountTokens("hi")
		long := CountTokens("hello world this is a much longer text")
		if long <= short {
			t.Errorf("longer text should have more tokens: short=%d, long=%d", short, long)
		}
	})

	t.Run("consistent results", func(t *testing.T) {
		text := "The quick brown fox jumps over the lazy dog."
		result1 := CountTokens(text)
		result2 := CountTokens(text)
		if result1 != result2 {
			t.Errorf("inconsistent results: %d != %d", result1, result2)
		}
	})
}

func TestCountTokensCodeContent(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		minTok int // minimum expected tokens
	}{
		{"function signature", "func main() {}", 3},
		{"variable assignment", "x := 42", 3},
		{"import statement", `import "fmt"`, 3},
		{"method call", "fmt.Println(x)", 3},
		{"go struct", "type User struct { Name string }", 5},
		{"slice literal", "[]int{1, 2, 3}", 5},
		{"map literal", `map[string]int{"a": 1}`, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountTokens(tt.text)
			if result < tt.minTok {
				t.Errorf("CountTokens(%q) = %d, want >= %d", tt.text, result, tt.minTok)
			}
		})
	}
}

func TestCountTokensUnicode(t *testing.T) {
	t.Run("CJK characters are tokenized", func(t *testing.T) {
		// Chinese, Japanese, Korean should all produce tokens
		texts := []string{"你好", "日本語", "안녕하세요"}
		for _, text := range texts {
			result := CountTokens(text)
			if result < 1 {
				t.Errorf("CountTokens(%q) = %d, want >= 1", text, result)
			}
		}
	})

	t.Run("emoji produces tokens", func(t *testing.T) {
		result := CountTokens("😀")
		if result < 1 {
			t.Errorf("CountTokens(emoji) = %d, want >= 1", result)
		}
	})

	t.Run("multiple emoji produce more tokens", func(t *testing.T) {
		single := CountTokens("😀")
		multiple := CountTokens("😀😁😂")
		if multiple < single {
			t.Errorf("multiple emoji (%d) should have >= tokens than single (%d)", multiple, single)
		}
	})

	t.Run("RTL text is tokenized", func(t *testing.T) {
		arabic := CountTokens("مرحبا")
		if arabic < 1 {
			t.Errorf("Arabic text = %d tokens, want >= 1", arabic)
		}
	})

	t.Run("mixed scripts are tokenized", func(t *testing.T) {
		result := CountTokens("Hello 你好")
		if result < 2 {
			t.Errorf("Mixed scripts = %d tokens, want >= 2", result)
		}
	})

	t.Run("accented characters are handled", func(t *testing.T) {
		result := CountTokens("résumé")
		if result < 1 {
			t.Errorf("Accented text = %d tokens, want >= 1", result)
		}
	})

	t.Run("emoji with ZWJ is tokenized", func(t *testing.T) {
		// Family emoji with zero-width joiners
		result := CountTokens("👨‍👩‍👧")
		if result < 1 {
			t.Errorf("ZWJ emoji = %d tokens, want >= 1", result)
		}
	})
}

func TestCountTokensLongStrings(t *testing.T) {
	t.Run("repeated word", func(t *testing.T) {
		// "hello " repeated 1000 times
		text := strings.Repeat("hello ", 1000)
		result := CountTokens(text)
		// Should produce significant number of tokens
		if result < 500 {
			t.Errorf("CountTokens for 1000 'hello ' = %d, expected >= 500", result)
		}
	})

	t.Run("long paragraph", func(t *testing.T) {
		text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)
		result := CountTokens(text)
		// Should produce significant number of tokens
		if result < 500 {
			t.Errorf("CountTokens for long paragraph = %d, expected >= 500", result)
		}
	})

	t.Run("very long single line", func(t *testing.T) {
		// 10000 character string
		text := strings.Repeat("a", 10000)
		result := CountTokens(text)
		if result == 0 {
			t.Error("CountTokens returned 0 for long string")
		}
	})

	t.Run("code block", func(t *testing.T) {
		code := `
package main

import (
	"fmt"
	"strings"
)

func main() {
	message := "Hello, World!"
	words := strings.Split(message, " ")
	for _, word := range words {
		fmt.Println(word)
	}
}
`
		result := CountTokens(code)
		// Reasonable code block should produce tokens
		if result < 20 {
			t.Errorf("CountTokens for code block = %d, expected >= 20", result)
		}
	})
}

func TestEstimateTokensEquivalence(t *testing.T) {
	// EstimateTokens should return the same as CountTokens
	texts := []string{
		"",
		"hello",
		"hello world",
		"func main() {}",
		strings.Repeat("test ", 100),
	}

	for _, text := range texts {
		count := CountTokens(text)
		estimate := EstimateTokens(text)
		if count != estimate {
			t.Errorf("CountTokens(%q) = %d, EstimateTokens = %d (should be equal)",
				text, count, estimate)
		}
	}
}

func TestEstimateTokensBytes(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{"empty", []byte{}},
		{"ascii", []byte("hello world")},
		{"unicode", []byte("你好世界")},
		{"binary-like", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}},
		{"mixed", []byte("hello\x00world")}, // null byte in middle
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytesResult := EstimateTokensBytes(tt.content)
			stringResult := CountTokens(string(tt.content))
			if bytesResult != stringResult {
				t.Errorf("EstimateTokensBytes = %d, CountTokens(string()) = %d (should match)",
					bytesResult, stringResult)
			}
		})
	}
}

func TestTokenCountingConcurrency(t *testing.T) {
	// Test that the tokenizer singleton is thread-safe
	const goroutines = 50
	const iterations = 100

	texts := []string{
		"hello world",
		"func main() {}",
		"你好世界",
		strings.Repeat("test ", 50),
	}

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				text := texts[(id+j)%len(texts)]
				count1 := CountTokens(text)
				count2 := CountTokens(text)
				if count1 != count2 {
					errors <- &tokenError{text: text, count1: count1, count2: count2}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent counting mismatch: %v", err)
	}
}

type tokenError struct {
	text   string
	count1 int
	count2 int
}

func (e *tokenError) Error() string {
	return "token count mismatch"
}

func TestCountTokensEdgeCases(t *testing.T) {
	t.Run("null bytes", func(t *testing.T) {
		text := "hello\x00world"
		result := CountTokens(text)
		if result < 2 {
			t.Errorf("CountTokens with null byte = %d, expected at least 2", result)
		}
	})

	t.Run("control characters", func(t *testing.T) {
		text := "hello\x01\x02\x03world"
		result := CountTokens(text)
		// Should handle control characters gracefully
		if result < 2 {
			t.Errorf("CountTokens with control chars = %d, expected at least 2", result)
		}
	})

	t.Run("surrogate pairs", func(t *testing.T) {
		// U+1F600 (grinning face) encoded correctly
		text := "😀"
		result := CountTokens(text)
		if result == 0 {
			t.Error("CountTokens returned 0 for emoji")
		}
	})

	t.Run("BOM", func(t *testing.T) {
		// UTF-8 BOM
		text := "\xEF\xBB\xBFhello"
		result := CountTokens(text)
		if result < 1 {
			t.Errorf("CountTokens with BOM = %d, expected at least 1", result)
		}
	})

	t.Run("markdown formatting", func(t *testing.T) {
		text := "# Heading\n\n**bold** and *italic*"
		result := CountTokens(text)
		if result < 5 {
			t.Errorf("CountTokens for markdown = %d, expected at least 5", result)
		}
	})

	t.Run("json content", func(t *testing.T) {
		text := `{"key": "value", "number": 42, "array": [1, 2, 3]}`
		result := CountTokens(text)
		if result < 10 {
			t.Errorf("CountTokens for JSON = %d, expected at least 10", result)
		}
	})

	t.Run("xml content", func(t *testing.T) {
		text := `<root><child attr="value">content</child></root>`
		result := CountTokens(text)
		if result < 8 {
			t.Errorf("CountTokens for XML = %d, expected at least 8", result)
		}
	})

	t.Run("sql query", func(t *testing.T) {
		text := `SELECT * FROM users WHERE id = 1 AND status = 'active' ORDER BY created_at DESC`
		result := CountTokens(text)
		if result < 10 {
			t.Errorf("CountTokens for SQL = %d, expected at least 10", result)
		}
	})
}

func BenchmarkCountTokens(b *testing.B) {
	texts := []struct {
		name string
		text string
	}{
		{"short", "hello world"},
		{"medium", strings.Repeat("hello world ", 100)},
		{"long", strings.Repeat("hello world ", 1000)},
		{"code", `func main() { fmt.Println("Hello, World!") }`},
	}

	for _, tt := range texts {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				CountTokens(tt.text)
			}
		})
	}
}

func BenchmarkEstimateTokensBytes(b *testing.B) {
	content := []byte(strings.Repeat("hello world ", 100))
	for i := 0; i < b.N; i++ {
		EstimateTokensBytes(content)
	}
}
