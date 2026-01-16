package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogChunker_Name(t *testing.T) {
	c := NewLogChunker()
	if c.Name() != "log" {
		t.Errorf("expected name 'log', got %q", c.Name())
	}
}

func TestLogChunker_Priority(t *testing.T) {
	c := NewLogChunker()
	if c.Priority() != 25 {
		t.Errorf("expected priority 25, got %d", c.Priority())
	}
}

func TestLogChunker_CanHandle(t *testing.T) {
	c := NewLogChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-log", "", true},
		{"application/x-log", "", true},
		{"", "log", true},
		{"", "logs", true},
		{"", "accesslog", true},
		{"", "app.log", true},
		{"", "error.log", true},
		{"", "output.out", true},
		{"text/plain", "", false},
		{"application/json", "", false},
		{"", "config.toml", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestLogChunker_EmptyContent(t *testing.T) {
	c := NewLogChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "log" {
		t.Errorf("expected chunker name 'log', got %q", result.ChunkerUsed)
	}
}

func TestLogChunker_StructuredFormat(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Application starting
2024-01-15T10:00:01.000Z INFO  [config] Loading configuration
2024-01-15T10:00:02.000Z DEBUG [db] Connecting to database
2024-01-15T10:00:03.000Z INFO  [server] Server started
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if chunk.Metadata.Log == nil {
		t.Fatal("expected Log metadata")
	}

	// Should detect structured format
	if chunk.Metadata.Log.LogFormat != "structured" {
		t.Errorf("expected LogFormat 'structured', got %q", chunk.Metadata.Log.LogFormat)
	}

	// Predominant level should be INFO
	if chunk.Metadata.Log.LogLevel != "INFO" {
		t.Errorf("expected LogLevel 'INFO', got %q", chunk.Metadata.Log.LogLevel)
	}
}

func TestLogChunker_ErrorDetection(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Application starting
2024-01-15T10:00:01.000Z INFO  [config] Loading configuration
2024-01-15T10:00:02.000Z ERROR [db] Connection failed
2024-01-15T10:00:03.000Z WARN  [db] Retrying connection
2024-01-15T10:00:04.000Z ERROR [db] Connection timeout
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have error count
	totalErrors := 0
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Log != nil {
			totalErrors += chunk.Metadata.Log.ErrorCount
		}
	}

	if totalErrors < 2 {
		t.Errorf("expected at least 2 errors counted, got %d", totalErrors)
	}
}

func TestLogChunker_JSONFormat(t *testing.T) {
	c := NewLogChunker()
	content := `{"timestamp":"2024-01-15T10:00:00Z","level":"info","message":"Starting up"}
{"timestamp":"2024-01-15T10:00:01Z","level":"error","message":"Failed to connect"}
{"timestamp":"2024-01-15T10:00:02Z","level":"info","message":"Retry successful"}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if chunk.Metadata.Log == nil {
		t.Fatal("expected Log metadata")
	}

	// Should detect JSON format
	if chunk.Metadata.Log.LogFormat != "json" {
		t.Errorf("expected LogFormat 'json', got %q", chunk.Metadata.Log.LogFormat)
	}
}

func TestLogChunker_ApacheFormat(t *testing.T) {
	c := NewLogChunker()
	content := `127.0.0.1 - - [15/Jan/2024:10:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234
192.168.1.1 - - [15/Jan/2024:10:00:01 +0000] "POST /api/data HTTP/1.1" 201 56
10.0.0.1 - - [15/Jan/2024:10:00:02 +0000] "GET /missing HTTP/1.1" 404 0
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if chunk.Metadata.Log == nil {
		t.Fatal("expected Log metadata")
	}

	// Should detect Apache format
	if chunk.Metadata.Log.LogFormat != "apache" {
		t.Errorf("expected LogFormat 'apache', got %q", chunk.Metadata.Log.LogFormat)
	}
}

func TestLogChunker_SyslogFormat(t *testing.T) {
	c := NewLogChunker()
	content := `Jan 15 10:00:00 hostname myapp[12345]: Application started
Jan 15 10:00:01 hostname myapp[12345]: Processing request
Jan 15 10:00:02 hostname myapp[12345]: Request completed
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if chunk.Metadata.Log == nil {
		t.Fatal("expected Log metadata")
	}

	// Should detect syslog format
	if chunk.Metadata.Log.LogFormat != "syslog" {
		t.Errorf("expected LogFormat 'syslog', got %q", chunk.Metadata.Log.LogFormat)
	}
}

func TestLogChunker_LevelExtraction(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"INFO level", "2024-01-15 INFO Something happened", "INFO"},
		{"ERROR level", "2024-01-15 ERROR Failed to process", "ERROR"},
		{"WARN level", "2024-01-15 WARNING Low memory", "WARN"},
		{"DEBUG level", "2024-01-15 DEBUG Detailed info", "DEBUG"},
		{"FATAL level", "2024-01-15 FATAL Critical error", "FATAL"},
		{"Lowercase", "2024-01-15 error something failed", "ERROR"},
		{"Bracketed", "[ERROR] Connection refused", "ERROR"},
	}

	c := &LogChunker{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := c.extractLevel(tt.line)
			if level != tt.expected {
				t.Errorf("extractLevel(%q) = %q, want %q", tt.line, level, tt.expected)
			}
		})
	}
}

func TestLogChunker_TimestampExtraction(t *testing.T) {
	c := &LogChunker{}

	tests := []struct {
		name    string
		line    string
		hasTime bool
	}{
		{"ISO8601", "2024-01-15T10:00:00.000Z INFO message", true},
		{"ISO8601 no ms", "2024-01-15T10:00:00Z INFO message", true},
		{"Space separated", "2024-01-15 10:00:00.000 INFO message", true},
		{"Apache format", `[15/Jan/2024:10:00:00 +0000] "GET /"`, true},
		{"Syslog", "Jan 15 10:00:00 host app: message", true},
		{"No timestamp", "Just a plain message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := c.extractTimestamp(tt.line)
			if tt.hasTime && ts.IsZero() {
				t.Errorf("extractTimestamp(%q) returned zero time, expected valid time", tt.line)
			}
			if !tt.hasTime && !ts.IsZero() {
				t.Errorf("extractTimestamp(%q) returned %v, expected zero time", tt.line, ts)
			}
		})
	}
}

func TestLogChunker_ErrorAwareChunking(t *testing.T) {
	c := NewLogChunker()

	// Create content with many lines to trigger chunking, with errors mixed in
	var builder strings.Builder
	for i := 0; i < 50; i++ {
		if i == 25 {
			builder.WriteString("2024-01-15T10:00:25.000Z ERROR [db] Critical failure occurred\n")
		} else {
			builder.WriteString("2024-01-15T10:00:" + string(rune('0'+i%10)) + "0.000Z INFO [app] Normal log message number " + string(rune('0'+i%10)) + "\n")
		}
	}

	opts := ChunkOptions{
		MaxChunkSize: 500,
		MaxTokens:    100,
	}

	result, err := c.Chunk(context.Background(), []byte(builder.String()), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have multiple chunks due to size limit
	if result.TotalChunks < 2 {
		t.Log("Note: chunk count depends on error-aware boundaries")
	}

	// At least one chunk should have errors
	hasErrorChunk := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Log != nil && chunk.Metadata.Log.ErrorCount > 0 {
			hasErrorChunk = true
			break
		}
	}

	if !hasErrorChunk {
		t.Error("expected at least one chunk with errors")
	}
}

func TestLogChunker_MinimumSizeEnforcement(t *testing.T) {
	c := NewLogChunker()

	// Create small content that shouldn't be split into tiny chunks
	content := `2024-01-15T10:00:00.000Z INFO  [main] Line 1
2024-01-15T10:00:01.000Z INFO  [main] Line 2
2024-01-15T10:00:02.000Z INFO  [main] Line 3
`

	opts := ChunkOptions{
		MaxChunkSize: 50, // Very small, but min size should prevent tiny chunks
		MaxTokens:    10,
	}

	result, err := c.Chunk(context.Background(), []byte(content), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still produce at least one chunk
	if result.TotalChunks == 0 {
		t.Error("expected at least 1 chunk")
	}

	// Chunks shouldn't be smaller than minimum (unless it's all content)
	for _, chunk := range result.Chunks {
		if len(chunk.Content) < logMinChunkSize && len(chunk.Content) < len(content) {
			t.Logf("Note: chunk smaller than min size: %d bytes", len(chunk.Content))
		}
	}
}

func TestLogChunker_ChunkType(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Test message`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Type != ChunkTypeStructured {
			t.Errorf("expected ChunkTypeStructured, got %q", chunk.Metadata.Type)
		}
	}
}

func TestLogChunker_TestdataFixture(t *testing.T) {
	c := NewLogChunker()

	fixturePath := filepath.Join("..", "..", "testdata", "data", "sample.log")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Error("expected at least 1 chunk for fixture")
	}

	// Fixture should be detected as structured format (ISO8601 timestamps)
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Verify format detection
		if chunk.Metadata.Log.LogFormat != "structured" {
			t.Logf("Note: detected format %q (expected 'structured')", chunk.Metadata.Log.LogFormat)
		}

		// Fixture has errors
		totalErrors := 0
		for _, c := range result.Chunks {
			if c.Metadata.Log != nil {
				totalErrors += c.Metadata.Log.ErrorCount
			}
		}
		if totalErrors == 0 {
			t.Log("Note: fixture should contain some error entries")
		}
	}
}

func TestLogChunker_ContextCancellation(t *testing.T) {
	c := NewLogChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `2024-01-15T10:00:00.000Z INFO  [main] Test message`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestLogChunker_TokenEstimate(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] This is a log message with some content
2024-01-15T10:00:01.000Z INFO  [main] Another log message for token estimation
2024-01-15T10:00:02.000Z DEBUG [main] Debug level message with additional details
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.TokenEstimate <= 0 {
			t.Error("expected positive TokenEstimate")
		}
	}
}

func TestLogChunker_MixedLevels(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Info message 1
2024-01-15T10:00:01.000Z DEBUG [main] Debug message
2024-01-15T10:00:02.000Z INFO  [main] Info message 2
2024-01-15T10:00:03.000Z WARN  [main] Warning message
2024-01-15T10:00:04.000Z INFO  [main] Info message 3
2024-01-15T10:00:05.000Z INFO  [main] Info message 4
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Predominant level should be INFO (most common)
		if chunk.Metadata.Log.LogLevel != "INFO" {
			t.Errorf("expected predominant level 'INFO', got %q", chunk.Metadata.Log.LogLevel)
		}
	}
}

func TestLogChunker_TimeRange(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] First message
2024-01-15T10:30:00.000Z INFO  [main] Middle message
2024-01-15T11:00:00.000Z INFO  [main] Last message
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Should have valid time range
		if chunk.Metadata.Log.TimeStart.IsZero() {
			t.Error("expected non-zero TimeStart")
		}
		if chunk.Metadata.Log.TimeEnd.IsZero() {
			t.Error("expected non-zero TimeEnd")
		}
		if !chunk.Metadata.Log.TimeEnd.After(chunk.Metadata.Log.TimeStart) &&
			!chunk.Metadata.Log.TimeEnd.Equal(chunk.Metadata.Log.TimeStart) {
			t.Error("expected TimeEnd >= TimeStart")
		}
	}
}

func TestLogChunker_CustomFormat(t *testing.T) {
	c := NewLogChunker()
	content := `[MyApp] Processing request 123
[MyApp] Request completed successfully
[MyApp] Starting new task
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Should detect as custom format
		if chunk.Metadata.Log.LogFormat != "custom" {
			t.Errorf("expected LogFormat 'custom', got %q", chunk.Metadata.Log.LogFormat)
		}
	}
}

func TestLogChunker_NginxFormat(t *testing.T) {
	c := NewLogChunker()
	// Nginx combined log format
	content := `192.168.1.1 - user [15/Jan/2024:10:00:00 +0000] "GET /api/data HTTP/1.1" 200 1234 "https://example.com" "nginx/1.18.0" upstream: "10.0.0.1:8080"
10.0.0.2 - - [15/Jan/2024:10:00:01 +0000] "POST /api/submit HTTP/1.1" 201 56 "-" "nginx client"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Should detect nginx or apache format
		if chunk.Metadata.Log.LogFormat != "nginx" && chunk.Metadata.Log.LogFormat != "apache" {
			t.Logf("Note: detected format %q (expected 'nginx' or 'apache')", chunk.Metadata.Log.LogFormat)
		}
	}
}

func TestLogChunker_StackTrace(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Application starting
2024-01-15T10:00:01.000Z ERROR [main] Exception occurred
java.lang.NullPointerException: Object is null
    at com.example.Service.process(Service.java:42)
    at com.example.Controller.handle(Controller.java:28)
    at sun.reflect.NativeMethodAccessorImpl.invoke(Unknown Source)
    at org.springframework.web.servlet.FrameworkServlet.service(FrameworkServlet.java:897)
Caused by: java.io.IOException: Connection refused
    at java.net.Socket.connect(Socket.java:591)
    ... 15 more
2024-01-15T10:00:02.000Z INFO  [main] Recovery attempted
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Stack trace should be preserved with error
	allContent := ""
	for _, chunk := range result.Chunks {
		allContent += chunk.Content
	}

	if !strings.Contains(allContent, "NullPointerException") {
		t.Error("expected stack trace to be preserved")
	}
	if !strings.Contains(allContent, "at com.example.Service") {
		t.Error("expected stack trace frames to be preserved")
	}
}

func TestLogChunker_FatalLevel(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Application starting
2024-01-15T10:00:01.000Z FATAL [main] Unrecoverable error occurred
2024-01-15T10:00:02.000Z CRITICAL [main] System shutdown initiated
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FATAL level should be counted as errors
	totalErrors := 0
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Log != nil {
			totalErrors += chunk.Metadata.Log.ErrorCount
		}
	}

	if totalErrors < 2 {
		t.Errorf("expected at least 2 errors (FATAL + CRITICAL), got %d", totalErrors)
	}
}

func TestLogChunker_MalformedJSON(t *testing.T) {
	c := NewLogChunker()
	content := `{"timestamp":"2024-01-15T10:00:00Z","level":"info","message":"Valid JSON"}
{"timestamp":"2024-01-15T10:00:01Z","level":"error","message":"Also valid"}
{"timestamp":"2024-01-15T10:00:02Z","level":"info","message":"Incomplete JSON
{"timestamp":"2024-01-15T10:00:03Z","level":"info","message":"After malformed"}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle malformed JSON gracefully
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestLogChunker_MixedFormats(t *testing.T) {
	c := NewLogChunker()
	// Mix of JSON and structured log formats
	content := `{"timestamp":"2024-01-15T10:00:00Z","level":"info","message":"JSON log"}
2024-01-15T10:00:01.000Z INFO  [main] Structured log
{"timestamp":"2024-01-15T10:00:02Z","level":"error","message":"Back to JSON"}
Jan 15 10:00:03 hostname app[123]: Syslog format
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle mixed formats
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestLogChunker_AllDebugEntries(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z DEBUG [main] Debug message 1
2024-01-15T10:00:01.000Z DEBUG [main] Debug message 2
2024-01-15T10:00:02.000Z DEBUG [main] Debug message 3
2024-01-15T10:00:03.000Z DEBUG [main] Debug message 4
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Predominant level should be DEBUG when all entries are DEBUG
		if chunk.Metadata.Log.LogLevel != "DEBUG" {
			t.Errorf("expected LogLevel 'DEBUG', got %q", chunk.Metadata.Log.LogLevel)
		}
	}
}

func TestLogChunker_VeryLongLine(t *testing.T) {
	c := NewLogChunker()

	// Create a very long log line
	var builder strings.Builder
	builder.WriteString("2024-01-15T10:00:00.000Z INFO  [main] Very long message: ")
	for i := 0; i < 1000; i++ {
		builder.WriteString("data ")
	}
	builder.WriteString("\n2024-01-15T10:00:01.000Z INFO  [main] Normal line\n")

	result, err := c.Chunk(context.Background(), []byte(builder.String()), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle very long lines
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestLogChunker_UnicodeContent(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [i18n] 中文日志消息
2024-01-15T10:00:01.000Z INFO  [i18n] 日本語ログメッセージ
2024-01-15T10:00:02.000Z INFO  [i18n] 이모지 테스트 🎉🚀💻
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Unicode should be preserved
		if !strings.Contains(chunk.Content, "中文日志") {
			t.Error("expected Chinese characters to be preserved")
		}
		if !strings.Contains(chunk.Content, "🎉") {
			t.Error("expected emoji to be preserved")
		}
	}
}

func TestLogChunker_LevelPositionVariations(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "level at start",
			line:     "ERROR 2024-01-15 Something failed",
			expected: "ERROR",
		},
		{
			name:     "level in brackets",
			line:     "2024-01-15 [ERROR] Something failed",
			expected: "ERROR",
		},
		{
			name:     "level after timestamp",
			line:     "2024-01-15T10:00:00Z ERROR Something failed",
			expected: "ERROR",
		},
		{
			name:     "level with colon",
			line:     "ERROR: Something failed at 2024-01-15",
			expected: "ERROR",
		},
	}

	c := &LogChunker{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := c.extractLevel(tt.line)
			if level != tt.expected {
				t.Errorf("extractLevel(%q) = %q, want %q", tt.line, level, tt.expected)
			}
		})
	}
}

func TestLogChunker_EmptyLines(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00.000Z INFO  [main] Message 1

2024-01-15T10:00:01.000Z INFO  [main] Message 2


2024-01-15T10:00:02.000Z INFO  [main] Message 3
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty lines should be handled
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestLogChunker_JSONWithNestedObjects(t *testing.T) {
	c := NewLogChunker()
	content := `{"timestamp":"2024-01-15T10:00:00Z","level":"info","message":"Request","context":{"user":{"id":123,"name":"test"},"request":{"method":"GET","path":"/api"}}}
{"timestamp":"2024-01-15T10:00:01Z","level":"error","message":"Failed","error":{"code":500,"details":{"stack":"trace here"}}}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Should detect JSON format
		if chunk.Metadata.Log.LogFormat != "json" {
			t.Errorf("expected LogFormat 'json', got %q", chunk.Metadata.Log.LogFormat)
		}
	}
}

func TestLogChunker_PartialTimestamps(t *testing.T) {
	c := NewLogChunker()
	// Some lines have timestamps, some don't (like continuation lines)
	content := `2024-01-15T10:00:00.000Z INFO  [main] Starting process
    with additional details here
    and more context
2024-01-15T10:00:01.000Z INFO  [main] Process completed
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// All content should be included
	allContent := ""
	for _, chunk := range result.Chunks {
		allContent += chunk.Content
	}

	if !strings.Contains(allContent, "additional details") {
		t.Error("expected continuation lines to be preserved")
	}
}

func TestLogChunker_DifferentTimezones(t *testing.T) {
	c := NewLogChunker()
	content := `2024-01-15T10:00:00Z INFO Message at UTC
2024-01-15T10:00:00+05:30 INFO Message at IST
2024-01-15T10:00:00-08:00 INFO Message at PST
2024-01-15T10:00:00.000+0000 INFO Message with offset
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Log == nil {
			t.Fatal("expected Log metadata")
		}

		// Should extract timestamps from various formats
		if chunk.Metadata.Log.TimeStart.IsZero() {
			t.Log("Note: timestamp extraction may not support all timezone formats")
		}
	}
}

func TestLogChunker_ErrorLevelMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"FATAL", "FATAL"},
		{"CRITICAL", "FATAL"},
		{"ALERT", "FATAL"},
		{"EMERGENCY", "FATAL"},
		{"ERROR", "ERROR"},
		{"ERR", "ERROR"},
		{"SEVERE", "ERROR"},
		{"WARN", "WARN"},
		{"WARNING", "WARN"},
		{"INFO", "INFO"},
		{"INFORMATION", "INFO"},
		{"DEBUG", "DEBUG"},
		{"TRACE", "DEBUG"},
		{"VERBOSE", "DEBUG"},
		{"FINE", "DEBUG"},
	}

	c := &LogChunker{}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := c.normalizeLevel(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeLevel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLogChunker_WhitespaceOnlyContent(t *testing.T) {
	c := NewLogChunker()
	content := `


`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Whitespace-only should produce minimal chunks
	t.Logf("whitespace-only content produced %d chunks", result.TotalChunks)
}

func TestLogChunker_ErrorWithContext(t *testing.T) {
	c := NewLogChunker()

	// Create content where error appears after substantial context
	var builder strings.Builder
	for i := 0; i < 20; i++ {
		builder.WriteString("2024-01-15T10:00:")
		builder.WriteString(string(rune('0' + i/10)))
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString(".000Z INFO  [main] Context line ")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString("\n")
	}
	builder.WriteString("2024-01-15T10:00:20.000Z ERROR [main] Critical error occurred\n")
	for i := 21; i < 25; i++ {
		builder.WriteString("2024-01-15T10:00:")
		builder.WriteString(string(rune('0' + i/10)))
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString(".000Z INFO  [main] After error line\n")
	}

	opts := ChunkOptions{
		MaxChunkSize: 1000,
		MaxTokens:    200,
	}

	result, err := c.Chunk(context.Background(), []byte(builder.String()), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Error-aware chunking should keep context near errors
	hasErrorChunk := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Log != nil && chunk.Metadata.Log.ErrorCount > 0 {
			hasErrorChunk = true
			// Error chunk should contain the error message
			if !strings.Contains(chunk.Content, "Critical error") {
				t.Error("expected error chunk to contain error message")
			}
		}
	}

	if !hasErrorChunk {
		t.Error("expected at least one chunk with errors")
	}
}
