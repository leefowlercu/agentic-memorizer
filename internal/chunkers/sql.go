package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	sqlChunkerName     = "sql"
	sqlChunkerPriority = 32
)

// SQL statement patterns
var (
	// DDL patterns
	sqlCreateTableRegex    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TEMP(?:ORARY)?\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlAlterTableRegex     = regexp.MustCompile(`(?i)^\s*ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlDropTableRegex      = regexp.MustCompile(`(?i)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlCreateIndexRegex    = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)\s+ON\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlCreateViewRegex     = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TEMP(?:ORARY)?\s+)?(?:MATERIALIZED\s+)?VIEW\s+(?:IF\s+NOT\s+EXISTS\s+)?(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlCreateTriggerRegex  = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?TRIGGER\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)(?:\s|\n)+(?:BEFORE|AFTER|INSTEAD\s+OF).*?ON\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)

	// Procedure/Function patterns
	sqlCreateFunctionRegex  = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlCreateProcedureRegex = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?PROCEDURE\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)

	// DML patterns
	sqlInsertRegex = regexp.MustCompile(`(?i)^\s*INSERT\s+INTO\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlUpdateRegex = regexp.MustCompile(`(?i)^\s*UPDATE\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlDeleteRegex = regexp.MustCompile(`(?i)^\s*DELETE\s+FROM\s+(["`+"`)"+`]?[\w.]+["`+"`)"+`]?)`)
	sqlSelectRegex = regexp.MustCompile(`(?i)^\s*SELECT\b`)

	// Dialect detection patterns
	sqlPostgresPatterns = []string{"SERIAL", "RETURNING", "::"}
	sqlMySQLPatterns    = []string{"AUTO_INCREMENT", "ENGINE=", "CHARSET="}
	sqlSQLitePatterns   = []string{"AUTOINCREMENT", "INTEGER PRIMARY KEY"}
	sqlSQLServerPatterns = []string{"IDENTITY(", "NVARCHAR", "TOP ", "WITH (NOLOCK)"}
	sqlOraclePatterns    = []string{"NUMBER(", "VARCHAR2", "NVL(", "DECODE("}
)

// SQLChunker splits SQL content by statements grouped by table.
type SQLChunker struct{}

// NewSQLChunker creates a new SQL chunker.
func NewSQLChunker() *SQLChunker {
	return &SQLChunker{}
}

// Name returns the chunker's identifier.
func (c *SQLChunker) Name() string {
	return sqlChunkerName
}

// CanHandle returns true for SQL content.
func (c *SQLChunker) CanHandle(mimeType string, language string) bool {
	mime := strings.ToLower(mimeType)
	lang := strings.ToLower(language)

	// Match by MIME type
	if mime == "application/sql" ||
		mime == "text/x-sql" ||
		mime == "application/x-sql" {
		return true
	}

	// Match by file extension
	if strings.HasSuffix(lang, ".sql") ||
		strings.HasSuffix(lang, ".ddl") ||
		strings.HasSuffix(lang, ".dml") {
		return true
	}

	// Match by language hint
	if lang == "sql" || lang == "plsql" || lang == "plpgsql" {
		return true
	}

	return false
}

// Priority returns the chunker's priority.
func (c *SQLChunker) Priority() int {
	return sqlChunkerPriority
}

// Chunk splits SQL content by statements.
func (c *SQLChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  sqlChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	dialect := c.detectDialect(text)
	statements := c.parseStatements(text)
	grouped := c.groupStatements(statements)

	var chunks []Chunk
	offset := 0

	for _, group := range grouped {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		groupContent := c.buildGroupContent(group)

		// If group is too large, split it further
		if len(groupContent) > maxSize {
			subChunks := c.splitLargeGroup(ctx, group, dialect, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(groupContent) != "" {
			// Determine primary metadata from first statement
			meta := c.extractMetadata(group[0], dialect)

			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     groupContent,
				StartOffset: offset,
				EndOffset:   offset + len(groupContent),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(groupContent),
					SQL:           meta,
				},
			})
		}

		offset += len(groupContent)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  sqlChunkerName,
		OriginalSize: len(content),
	}, nil
}

// sqlStatement represents a parsed SQL statement.
type sqlStatement struct {
	content       string
	statementType string
	objectType    string
	tableName     string
	procedureName string
}

// parseStatements extracts individual SQL statements from content.
func (c *SQLChunker) parseStatements(text string) []sqlStatement {
	var statements []sqlStatement
	var current strings.Builder
	var inString bool
	var stringChar rune
	var inBlockComment bool
	var inLineComment bool
	var inDollarQuote bool
	var dollarTag string

	lines := strings.Split(text, "\n")

	for _, line := range lines {
		runes := []rune(line)

		for i := 0; i < len(runes); i++ {
			ch := runes[i]
			nextCh := rune(0)
			if i+1 < len(runes) {
				nextCh = runes[i+1]
			}

			// Handle dollar quoting (PostgreSQL)
			if !inString && !inBlockComment && !inLineComment && ch == '$' {
				if inDollarQuote {
					// Check for closing tag
					if c.matchDollarTag(runes[i:], dollarTag) {
						current.WriteString(dollarTag)
						i += len([]rune(dollarTag)) - 1
						inDollarQuote = false
						continue
					}
				} else {
					// Check for opening tag
					tag := c.extractDollarTag(runes[i:])
					if tag != "" {
						current.WriteString(tag)
						i += len([]rune(tag)) - 1
						inDollarQuote = true
						dollarTag = tag
						continue
					}
				}
			}

			// Handle comments
			if !inString && !inDollarQuote {
				// Block comment start
				if ch == '/' && nextCh == '*' && !inBlockComment {
					inBlockComment = true
					current.WriteRune(ch)
					current.WriteRune(nextCh)
					i++
					continue
				}
				// Block comment end
				if ch == '*' && nextCh == '/' && inBlockComment {
					inBlockComment = false
					current.WriteRune(ch)
					current.WriteRune(nextCh)
					i++
					continue
				}
				// Line comment
				if ch == '-' && nextCh == '-' && !inLineComment && !inBlockComment {
					inLineComment = true
				}
			}

			// Handle strings
			if !inBlockComment && !inLineComment && !inDollarQuote {
				if ch == '\'' || ch == '"' {
					if !inString {
						inString = true
						stringChar = ch
					} else if ch == stringChar {
						// Check for escaped quote
						if nextCh == stringChar {
							current.WriteRune(ch)
							current.WriteRune(nextCh)
							i++
							continue
						}
						inString = false
					}
				}
			}

			current.WriteRune(ch)

			// Check for statement end
			if ch == ';' && !inString && !inBlockComment && !inLineComment && !inDollarQuote {
				stmt := current.String()
				if strings.TrimSpace(stmt) != "" {
					statements = append(statements, c.parseStatement(stmt))
				}
				current.Reset()
			}
		}

		// End of line
		current.WriteString("\n")
		inLineComment = false
	}

	// Capture any remaining content
	remaining := current.String()
	if strings.TrimSpace(remaining) != "" {
		statements = append(statements, c.parseStatement(remaining))
	}

	return statements
}

// extractDollarTag extracts a PostgreSQL dollar-quote tag like $$ or $tag$
func (c *SQLChunker) extractDollarTag(runes []rune) string {
	if len(runes) == 0 || runes[0] != '$' {
		return ""
	}

	var tag strings.Builder
	tag.WriteRune('$')

	for i := 1; i < len(runes); i++ {
		ch := runes[i]
		if ch == '$' {
			tag.WriteRune('$')
			return tag.String()
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' {
			tag.WriteRune(ch)
		} else {
			return ""
		}
	}

	return ""
}

// matchDollarTag checks if the remaining runes start with the given dollar tag
func (c *SQLChunker) matchDollarTag(runes []rune, tag string) bool {
	tagRunes := []rune(tag)
	if len(runes) < len(tagRunes) {
		return false
	}
	for i, r := range tagRunes {
		if runes[i] != r {
			return false
		}
	}
	return true
}

// parseStatement analyzes a single SQL statement.
func (c *SQLChunker) parseStatement(text string) sqlStatement {
	stmt := sqlStatement{content: text}
	upper := strings.ToUpper(strings.TrimSpace(text))

	// Detect statement type and extract metadata
	if match := sqlCreateTableRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "CREATE"
		stmt.objectType = "TABLE"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if match := sqlAlterTableRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "ALTER"
		stmt.objectType = "TABLE"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if match := sqlDropTableRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "DROP"
		stmt.objectType = "TABLE"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if match := sqlCreateIndexRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "CREATE"
		stmt.objectType = "INDEX"
		stmt.tableName = c.cleanIdentifier(match[2])
	} else if match := sqlCreateViewRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "CREATE"
		stmt.objectType = "VIEW"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if match := sqlCreateTriggerRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "CREATE"
		stmt.objectType = "TRIGGER"
		stmt.tableName = c.cleanIdentifier(match[2])
		stmt.procedureName = c.cleanIdentifier(match[1])
	} else if match := sqlCreateFunctionRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "CREATE"
		stmt.objectType = "FUNCTION"
		stmt.procedureName = c.cleanIdentifier(match[1])
	} else if match := sqlCreateProcedureRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "CREATE"
		stmt.objectType = "PROCEDURE"
		stmt.procedureName = c.cleanIdentifier(match[1])
	} else if match := sqlInsertRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "INSERT"
		stmt.objectType = "TABLE"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if match := sqlUpdateRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "UPDATE"
		stmt.objectType = "TABLE"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if match := sqlDeleteRegex.FindStringSubmatch(text); match != nil {
		stmt.statementType = "DELETE"
		stmt.objectType = "TABLE"
		stmt.tableName = c.cleanIdentifier(match[1])
	} else if sqlSelectRegex.MatchString(text) {
		stmt.statementType = "SELECT"
		stmt.objectType = "QUERY"
	} else if strings.HasPrefix(upper, "BEGIN") || strings.HasPrefix(upper, "COMMIT") ||
		strings.HasPrefix(upper, "ROLLBACK") {
		stmt.statementType = upper[:strings.IndexAny(upper+" ", " \t\n")]
		stmt.objectType = "TRANSACTION"
	} else if strings.HasPrefix(upper, "GRANT") || strings.HasPrefix(upper, "REVOKE") {
		stmt.statementType = upper[:strings.IndexAny(upper+" ", " \t\n")]
		stmt.objectType = "PERMISSION"
	} else {
		// Generic statement
		words := strings.Fields(upper)
		if len(words) > 0 {
			stmt.statementType = words[0]
		}
	}

	return stmt
}

// cleanIdentifier removes quotes and schema prefixes from an identifier.
func (c *SQLChunker) cleanIdentifier(id string) string {
	// Remove quotes
	id = strings.Trim(id, `"'` + "`" + `[]`)

	// Extract table name if schema-qualified
	parts := strings.Split(id, ".")
	return parts[len(parts)-1]
}

// groupStatements groups statements by table or as standalone procedures.
func (c *SQLChunker) groupStatements(statements []sqlStatement) [][]sqlStatement {
	var groups [][]sqlStatement
	tableGroups := make(map[string][]sqlStatement)
	var tableOrder []string
	var standaloneGroups [][]sqlStatement

	for _, stmt := range statements {
		// Procedures/functions are standalone
		if stmt.objectType == "FUNCTION" || stmt.objectType == "PROCEDURE" {
			standaloneGroups = append(standaloneGroups, []sqlStatement{stmt})
			continue
		}

		// Group by table name
		if stmt.tableName != "" {
			if _, exists := tableGroups[stmt.tableName]; !exists {
				tableOrder = append(tableOrder, stmt.tableName)
			}
			tableGroups[stmt.tableName] = append(tableGroups[stmt.tableName], stmt)
		} else {
			// Statements without table name (like SELECT, transaction control)
			standaloneGroups = append(standaloneGroups, []sqlStatement{stmt})
		}
	}

	// Add table groups in order
	for _, tableName := range tableOrder {
		groups = append(groups, tableGroups[tableName])
	}

	// Add standalone groups
	groups = append(groups, standaloneGroups...)

	return groups
}

// buildGroupContent builds the content string for a group of statements.
func (c *SQLChunker) buildGroupContent(group []sqlStatement) string {
	var builder strings.Builder
	for i, stmt := range group {
		builder.WriteString(stmt.content)
		if i < len(group)-1 && !strings.HasSuffix(strings.TrimSpace(stmt.content), "\n") {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

// extractMetadata extracts SQLMetadata from a statement.
func (c *SQLChunker) extractMetadata(stmt sqlStatement, dialect string) *SQLMetadata {
	return &SQLMetadata{
		StatementType: stmt.statementType,
		ObjectType:    stmt.objectType,
		TableName:     stmt.tableName,
		ProcedureName: stmt.procedureName,
		SQLDialect:    dialect,
	}
}

// detectDialect detects the SQL dialect from content patterns.
func (c *SQLChunker) detectDialect(text string) string {
	upper := strings.ToUpper(text)

	// Count pattern matches for each dialect
	scores := map[string]int{
		"postgresql": 0,
		"mysql":      0,
		"sqlite":     0,
		"sqlserver":  0,
		"oracle":     0,
	}

	for _, pattern := range sqlPostgresPatterns {
		if strings.Contains(upper, pattern) {
			scores["postgresql"]++
		}
	}
	for _, pattern := range sqlMySQLPatterns {
		if strings.Contains(upper, pattern) {
			scores["mysql"]++
		}
	}
	for _, pattern := range sqlSQLitePatterns {
		if strings.Contains(upper, pattern) {
			scores["sqlite"]++
		}
	}
	for _, pattern := range sqlSQLServerPatterns {
		if strings.Contains(upper, pattern) {
			scores["sqlserver"]++
		}
	}
	for _, pattern := range sqlOraclePatterns {
		if strings.Contains(upper, pattern) {
			scores["oracle"]++
		}
	}

	// Find dialect with highest score
	maxScore := 0
	dialect := ""
	for d, score := range scores {
		if score > maxScore {
			maxScore = score
			dialect = d
		}
	}

	// Return empty if no clear dialect detected
	if maxScore < 1 {
		return ""
	}

	return dialect
}

// splitLargeGroup splits a large group of statements into smaller chunks.
func (c *SQLChunker) splitLargeGroup(ctx context.Context, group []sqlStatement, dialect string, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk
	var current strings.Builder
	offset := baseOffset
	var firstStmt *sqlStatement

	for _, stmt := range group {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		// If adding this statement exceeds max, finalize current chunk
		if current.Len()+len(stmt.content) > maxSize && current.Len() > 0 {
			content := current.String()
			meta := c.extractMetadata(*firstStmt, dialect)
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset,
				EndOffset:   offset + len(content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeStructured,
					TokenEstimate: EstimateTokens(content),
					SQL:           meta,
				},
			})
			offset += len(content)
			current.Reset()
			firstStmt = nil
		}

		if firstStmt == nil {
			firstStmt = &stmt
		}

		current.WriteString(stmt.content)
		if !strings.HasSuffix(strings.TrimSpace(stmt.content), "\n") {
			current.WriteString("\n")
		}
	}

	// Finalize last chunk
	if current.Len() > 0 && firstStmt != nil {
		content := current.String()
		meta := c.extractMetadata(*firstStmt, dialect)
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset,
			EndOffset:   offset + len(content),
			Metadata: ChunkMetadata{
				Type:          ChunkTypeStructured,
				TokenEstimate: EstimateTokens(content),
				SQL:           meta,
			},
		})
	}

	return chunks
}
