package chunkers

import (
	"context"
	"regexp"
	"strings"
	"time"
)

const (
	logChunkerName     = "log"
	logChunkerPriority = 25

	// Minimum chunk size to prevent tiny chunks
	logMinChunkSize = 500
)

// Log format patterns
var (
	// JSON log format: {"timestamp":"...", "level":"...", ...}
	logJSONRegex = regexp.MustCompile(`^\s*\{.*"(?:level|severity|log_level)"\s*:\s*"[^"]*".*\}`)

	// Common structured log formats
	// ISO8601 timestamp followed by level: 2024-01-15T10:00:00.000Z INFO ...
	logISO8601Regex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d{3})?(?:Z|[+-]\d{2}:?\d{2})?)\s+(\w+)`)

	// Syslog format: Jan 15 10:00:00 hostname app[123]: message
	logSyslogRegex = regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[\d+\])?:\s*(.*)`)

	// Apache/NCSA Common Log Format: 127.0.0.1 - - [15/Jan/2024:10:00:00 +0000] "GET /path HTTP/1.1" 200 1234
	logApacheRegex = regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"([^"]+)"\s+(\d{3})`)

	// Nginx combined format (similar to Apache)
	logNginxRegex = regexp.MustCompile(`^(\S+)\s+-\s+\S+\s+\[([^\]]+)\]\s+"([^"]+)"\s+(\d{3})`)

	// Level patterns - ordered by specificity to avoid false matches
	logLevelPatterns = map[string]*regexp.Regexp{
		"FATAL": regexp.MustCompile(`(?i)\b(FATAL|CRITICAL)\b`),
		"ERROR": regexp.MustCompile(`(?i)\b(ERROR|ERR|SEVERE)\b`),
		"WARN":  regexp.MustCompile(`(?i)\b(WARN(?:ING)?)\b`),
		"DEBUG": regexp.MustCompile(`(?i)\b(DEBUG|TRACE|VERBOSE)\b`),
		"INFO":  regexp.MustCompile(`(?i)\b(INFO(?:RMATIONAL)?)\b`),
	}

	// Timestamp extraction patterns
	logTimestampPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d{3})?(?:Z|[+-]\d{2}:?\d{2})?)`),
		regexp.MustCompile(`(\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2}\s+[+-]\d{4})`),
		regexp.MustCompile(`([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})`),
	}
)

// LogChunker splits log files with error-aware chunking.
type LogChunker struct{}

// NewLogChunker creates a new log file chunker.
func NewLogChunker() *LogChunker {
	return &LogChunker{}
}

// Name returns the chunker's identifier.
func (c *LogChunker) Name() string {
	return logChunkerName
}

// CanHandle returns true for log file content.
func (c *LogChunker) CanHandle(mimeType string, language string) bool {
	mime := strings.ToLower(mimeType)
	lang := strings.ToLower(language)

	// Match by MIME type
	if mime == "text/x-log" || mime == "application/x-log" {
		return true
	}

	// Match by file extension
	if strings.HasSuffix(lang, ".log") ||
		strings.HasSuffix(lang, ".logs") ||
		strings.HasSuffix(lang, ".out") {
		return true
	}

	// Match by language hint
	if lang == "log" || lang == "logs" || lang == "accesslog" {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *LogChunker) Priority() int {
	return logChunkerPriority
}

// Chunk splits log content with error-aware boundaries.
func (c *LogChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  logChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	format := c.detectFormat(text)
	lines := strings.Split(text, "\n")

	var chunks []Chunk
	var current strings.Builder
	var chunkTimeStart, chunkTimeEnd time.Time
	var chunkErrorCount int
	var predominantLevel string
	levelCounts := make(map[string]int)
	offset := 0

	flushChunk := func() {
		if current.Len() == 0 {
			return
		}

		chunkContent := current.String()

		// Determine predominant level
		predominantLevel = "INFO"
		maxCount := 0
		for level, count := range levelCounts {
			if count > maxCount {
				maxCount = count
				predominantLevel = level
			}
		}

		chunks = append(chunks, Chunk{
			Index:       len(chunks),
			Content:     chunkContent,
			StartOffset: offset - len(chunkContent),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(chunkContent),
				Log: &LogMetadata{
					TimeStart:  chunkTimeStart,
					TimeEnd:    chunkTimeEnd,
					LogLevel:   predominantLevel,
					LogFormat:  format,
					ErrorCount: chunkErrorCount,
				},
			},
		})

		current.Reset()
		chunkTimeStart = time.Time{}
		chunkTimeEnd = time.Time{}
		chunkErrorCount = 0
		levelCounts = make(map[string]int)
	}

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		lineLen := len(line) + 1 // +1 for newline
		lineLevel := c.extractLevel(line)
		lineTime := c.extractTimestamp(line)

		// Track timestamps
		if !lineTime.IsZero() {
			if chunkTimeStart.IsZero() || lineTime.Before(chunkTimeStart) {
				chunkTimeStart = lineTime
			}
			if lineTime.After(chunkTimeEnd) {
				chunkTimeEnd = lineTime
			}
		}

		// Track level counts
		levelCounts[lineLevel]++

		// Count errors
		if lineLevel == "ERROR" || lineLevel == "FATAL" {
			chunkErrorCount++
		}

		// Check if we should flush before adding this line
		shouldFlush := false

		// Flush if adding this line would exceed max size
		if current.Len()+lineLen > maxSize && current.Len() >= logMinChunkSize {
			shouldFlush = true
		}

		// Error-aware flush: flush on ERROR/FATAL to keep errors with context
		// but only if we have enough content and the chunk is at reasonable size
		if (lineLevel == "ERROR" || lineLevel == "FATAL") &&
			current.Len() >= logMinChunkSize &&
			current.Len() > maxSize/2 {
			// Flush before the error to keep previous context separate
			// Then the error starts a new chunk with following context
			flushChunk()
			offset += 0 // offset already tracked
			shouldFlush = false
		}

		if shouldFlush {
			flushChunk()
		}

		current.WriteString(line)
		current.WriteString("\n")
		offset += lineLen
	}

	// Flush remaining content
	flushChunk()

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  logChunkerName,
		OriginalSize: len(content),
	}, nil
}

// detectFormat determines the log format from content.
func (c *LogChunker) detectFormat(text string) string {
	// Check first few non-empty lines
	lines := strings.Split(text, "\n")
	linesToCheck := 10
	if len(lines) < linesToCheck {
		linesToCheck = len(lines)
	}

	for i := 0; i < linesToCheck; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// JSON format
		if strings.HasPrefix(line, "{") && logJSONRegex.MatchString(line) {
			return "json"
		}

		// Apache/Nginx format
		if logApacheRegex.MatchString(line) || logNginxRegex.MatchString(line) {
			// Differentiate by typical nginx patterns
			if strings.Contains(line, `"nginx`) || strings.Contains(text, "upstream") {
				return "nginx"
			}
			return "apache"
		}

		// Syslog format
		if logSyslogRegex.MatchString(line) {
			return "syslog"
		}

		// ISO8601 structured format
		if logISO8601Regex.MatchString(line) {
			return "structured"
		}
	}

	// Custom/unknown format
	return "custom"
}

// extractLevel extracts the log level from a line.
func (c *LogChunker) extractLevel(line string) string {
	// Check each level pattern in priority order (most severe first, DEBUG before INFO to avoid false matches)
	levelOrder := []string{"FATAL", "ERROR", "WARN", "DEBUG", "INFO"}
	for _, level := range levelOrder {
		if pattern, exists := logLevelPatterns[level]; exists {
			if pattern.MatchString(line) {
				return level
			}
		}
	}

	// JSON log format - extract from field
	if strings.HasPrefix(strings.TrimSpace(line), "{") {
		levelMatch := regexp.MustCompile(`"(?:level|severity|log_level)"\s*:\s*"([^"]+)"`).FindStringSubmatch(line)
		if len(levelMatch) > 1 {
			return c.normalizeLevel(levelMatch[1])
		}
	}

	return "INFO" // Default to INFO if no level found
}

// normalizeLevel normalizes various level strings to standard values.
func (c *LogChunker) normalizeLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))

	switch level {
	case "FATAL", "CRITICAL", "ALERT", "EMERGENCY":
		return "FATAL"
	case "ERROR", "ERR", "SEVERE":
		return "ERROR"
	case "WARN", "WARNING":
		return "WARN"
	case "INFO", "INFORMATION", "INFORMATIONAL":
		return "INFO"
	case "DEBUG", "TRACE", "VERBOSE", "FINE", "FINEST":
		return "DEBUG"
	default:
		return level
	}
}

// extractTimestamp attempts to extract a timestamp from a log line.
func (c *LogChunker) extractTimestamp(line string) time.Time {
	for _, pattern := range logTimestampPatterns {
		match := pattern.FindStringSubmatch(line)
		if len(match) > 1 {
			ts := c.parseTimestamp(match[1])
			if !ts.IsZero() {
				return ts
			}
		}
	}

	// JSON format
	if strings.HasPrefix(strings.TrimSpace(line), "{") {
		tsMatch := regexp.MustCompile(`"(?:timestamp|time|@timestamp|ts)"\s*:\s*"([^"]+)"`).FindStringSubmatch(line)
		if len(tsMatch) > 1 {
			return c.parseTimestamp(tsMatch[1])
		}
	}

	return time.Time{}
}

// parseTimestamp parses various timestamp formats.
func (c *LogChunker) parseTimestamp(ts string) time.Time {
	// Try various formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"02/Jan/2006:15:04:05 -0700",
		"Jan _2 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t
		}
	}

	return time.Time{}
}
