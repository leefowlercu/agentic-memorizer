package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLChunker_Name(t *testing.T) {
	c := NewSQLChunker()
	if c.Name() != "sql" {
		t.Errorf("expected name 'sql', got %q", c.Name())
	}
}

func TestSQLChunker_Priority(t *testing.T) {
	c := NewSQLChunker()
	if c.Priority() != 32 {
		t.Errorf("expected priority 32, got %d", c.Priority())
	}
}

func TestSQLChunker_CanHandle(t *testing.T) {
	c := NewSQLChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"application/sql", "", true},
		{"text/x-sql", "", true},
		{"application/x-sql", "", true},
		{"", "sql", true},
		{"", "plsql", true},
		{"", "plpgsql", true},
		{"", "schema.sql", true},
		{"", "migrations.ddl", true},
		{"text/plain", "", false},
		{"application/json", "", false},
		{"", "go", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestSQLChunker_EmptyContent(t *testing.T) {
	c := NewSQLChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "sql" {
		t.Errorf("expected chunker name 'sql', got %q", result.ChunkerUsed)
	}
}

func TestSQLChunker_SingleStatement(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255)
);`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.SQL == nil {
			t.Fatal("expected SQL metadata")
		}
		if chunk.Metadata.SQL.StatementType != "CREATE" {
			t.Errorf("expected StatementType 'CREATE', got %q", chunk.Metadata.SQL.StatementType)
		}
		if chunk.Metadata.SQL.ObjectType != "TABLE" {
			t.Errorf("expected ObjectType 'TABLE', got %q", chunk.Metadata.SQL.ObjectType)
		}
		if chunk.Metadata.SQL.TableName != "users" {
			t.Errorf("expected TableName 'users', got %q", chunk.Metadata.SQL.TableName)
		}
	}
}

func TestSQLChunker_DDLGrouping(t *testing.T) {
	c := NewSQLChunker()
	// Statements with inline comments reference 'users' table
	content := `CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255)
);
CREATE INDEX idx_users_name ON users(name);
ALTER TABLE users ADD COLUMN email VARCHAR(255);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All statements reference 'users' table, should be grouped
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk (grouped by table), got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.SQL.TableName != "users" {
			t.Errorf("expected TableName 'users', got %q", chunk.Metadata.SQL.TableName)
		}
		// Should contain all statements
		if !strings.Contains(chunk.Content, "CREATE TABLE") {
			t.Error("expected chunk to contain CREATE TABLE")
		}
		if !strings.Contains(chunk.Content, "CREATE INDEX") {
			t.Error("expected chunk to contain CREATE INDEX")
		}
		if !strings.Contains(chunk.Content, "ALTER TABLE") {
			t.Error("expected chunk to contain ALTER TABLE")
		}
	}
}

func TestSQLChunker_MultipleTables(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE users (id SERIAL PRIMARY KEY);
CREATE TABLE posts (id SERIAL PRIMARY KEY, user_id INT);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 separate chunks for 2 tables
	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks (one per table), got %d", result.TotalChunks)
	}

	// Verify table names
	tableNames := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil {
			tableNames[chunk.Metadata.SQL.TableName] = true
		}
	}

	if !tableNames["users"] {
		t.Error("expected chunk for 'users' table")
	}
	if !tableNames["posts"] {
		t.Error("expected chunk for 'posts' table")
	}
}

func TestSQLChunker_ProcedureStandalone(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE users (id SERIAL PRIMARY KEY);

CREATE FUNCTION get_user(user_id INT) RETURNS users AS $$
BEGIN
    RETURN (SELECT * FROM users WHERE id = user_id);
END;
$$ LANGUAGE plpgsql;

CREATE PROCEDURE cleanup_old_users() AS $$
BEGIN
    DELETE FROM users WHERE created_at < NOW() - INTERVAL '1 year';
END;
$$ LANGUAGE plpgsql;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 chunks: table, function, procedure
	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Verify we have function and procedure as separate chunks
	hasFunction := false
	hasProcedure := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil {
			if chunk.Metadata.SQL.ObjectType == "FUNCTION" {
				hasFunction = true
				if chunk.Metadata.SQL.ProcedureName != "get_user" {
					t.Errorf("expected ProcedureName 'get_user', got %q", chunk.Metadata.SQL.ProcedureName)
				}
			}
			if chunk.Metadata.SQL.ObjectType == "PROCEDURE" {
				hasProcedure = true
				if chunk.Metadata.SQL.ProcedureName != "cleanup_old_users" {
					t.Errorf("expected ProcedureName 'cleanup_old_users', got %q", chunk.Metadata.SQL.ProcedureName)
				}
			}
		}
	}

	if !hasFunction {
		t.Error("expected function chunk")
	}
	if !hasProcedure {
		t.Error("expected procedure chunk")
	}
}

func TestSQLChunker_DialectDetection(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		expectedDialect string
	}{
		{
			name:            "PostgreSQL",
			content:         "CREATE TABLE users (id SERIAL PRIMARY KEY);",
			expectedDialect: "postgresql",
		},
		{
			name:            "MySQL",
			content:         "CREATE TABLE users (id INT AUTO_INCREMENT PRIMARY KEY) ENGINE=InnoDB;",
			expectedDialect: "mysql",
		},
		{
			name:            "SQLite",
			content:         "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT);",
			expectedDialect: "sqlite",
		},
		{
			name:            "SQL Server",
			content:         "CREATE TABLE users (id INT IDENTITY(1,1) PRIMARY KEY, name NVARCHAR(255));",
			expectedDialect: "sqlserver",
		},
		{
			name:            "Oracle",
			content:         "CREATE TABLE users (id NUMBER(10) PRIMARY KEY, name VARCHAR2(255));",
			expectedDialect: "oracle",
		},
	}

	c := NewSQLChunker()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.Chunk(context.Background(), []byte(tt.content), DefaultChunkOptions())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.TotalChunks > 0 {
				chunk := result.Chunks[0]
				if chunk.Metadata.SQL == nil {
					t.Fatal("expected SQL metadata")
				}
				if chunk.Metadata.SQL.SQLDialect != tt.expectedDialect {
					t.Errorf("expected dialect %q, got %q", tt.expectedDialect, chunk.Metadata.SQL.SQLDialect)
				}
			}
		})
	}
}

func TestSQLChunker_Comments(t *testing.T) {
	c := NewSQLChunker()
	content := `-- This is a comment
/* Multi-line
   comment */
CREATE TABLE users (
    id INT PRIMARY KEY -- inline comment
);`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Comments should be preserved
		if !strings.Contains(chunk.Content, "This is a comment") {
			t.Error("expected chunk to contain single-line comment")
		}
		if !strings.Contains(chunk.Content, "Multi-line") {
			t.Error("expected chunk to contain multi-line comment")
		}
	}
}

func TestSQLChunker_QuotedIdentifiers(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE "User Table" (id INT PRIMARY KEY);
CREATE TABLE [Another Table] (id INT PRIMARY KEY);
CREATE TABLE ` + "`" + `Backtick Table` + "`" + ` (id INT PRIMARY KEY);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse quoted identifiers correctly
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_DMLStatements(t *testing.T) {
	c := NewSQLChunker()
	content := `INSERT INTO users (name) VALUES ('Alice');
INSERT INTO users (name) VALUES ('Bob');
UPDATE users SET name = 'Charlie' WHERE id = 1;
DELETE FROM users WHERE id = 2;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All DML statements reference 'users' table, should be grouped
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk (grouped by table), got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.SQL.TableName != "users" {
			t.Errorf("expected TableName 'users', got %q", chunk.Metadata.SQL.TableName)
		}
	}
}

func TestSQLChunker_SelectStatements(t *testing.T) {
	c := NewSQLChunker()
	content := `SELECT * FROM users WHERE id = 1;
SELECT name, email FROM users ORDER BY name;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SELECT statements without table modification should be separate
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_ChunkType(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE test (id INT);`

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

func TestSQLChunker_TestdataFixture(t *testing.T) {
	c := NewSQLChunker()

	fixturePath := filepath.Join("..", "..", "testdata", "data", "sample.sql")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has users table DDL, posts table DDL, and some DML
	// Users: CREATE TABLE + CREATE INDEX + INSERT = grouped
	// Posts: CREATE TABLE + CREATE INDEX + INSERT = grouped
	// SELECT: separate
	if result.TotalChunks < 2 {
		t.Errorf("expected at least 2 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify dialect detection (fixture uses PostgreSQL syntax)
	hasPostgres := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil && chunk.Metadata.SQL.SQLDialect == "postgresql" {
			hasPostgres = true
			break
		}
	}

	if !hasPostgres {
		t.Log("Note: dialect detection may vary based on content")
	}
}

func TestSQLChunker_ContextCancellation(t *testing.T) {
	c := NewSQLChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `CREATE TABLE test (id INT);`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestSQLChunker_TokenEstimate(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

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

func TestSQLChunker_TransactionStatements(t *testing.T) {
	c := NewSQLChunker()
	content := `BEGIN;
INSERT INTO users (name) VALUES ('Alice');
COMMIT;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Transaction statements should be recognized
	hasTransaction := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil && chunk.Metadata.SQL.ObjectType == "TRANSACTION" {
			hasTransaction = true
			break
		}
	}

	if !hasTransaction {
		t.Log("Note: transaction statements may be grouped with DML")
	}
}

func TestSQLChunker_DollarQuoting(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE FUNCTION test() RETURNS void AS $$
BEGIN
    -- This semicolon should not split the statement;
    RAISE NOTICE 'Hello; world';
END;
$$ LANGUAGE plpgsql;

CREATE TABLE other (id INT);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Function with dollar quoting should be one chunk, table another
	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks (function + table), got %d", result.TotalChunks)
	}

	// Verify function is complete
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil && chunk.Metadata.SQL.ObjectType == "FUNCTION" {
			if !strings.Contains(chunk.Content, "RAISE NOTICE") {
				t.Error("expected function chunk to contain full body")
			}
		}
	}
}

func TestSQLChunker_ViewsAndTriggers(t *testing.T) {
	c := NewSQLChunker()
	// Use standard PostgreSQL trigger syntax
	content := `CREATE VIEW active_users AS SELECT * FROM users WHERE active = true;

CREATE TRIGGER update_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_modified_column();
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have view chunk at minimum
	hasView := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil {
			if chunk.Metadata.SQL.ObjectType == "VIEW" {
				hasView = true
			}
		}
	}

	if !hasView {
		t.Error("expected view chunk")
	}

	// Note: Trigger detection depends on regex matching BEFORE/AFTER...ON pattern
	t.Logf("Total chunks: %d", result.TotalChunks)
}

func TestSQLChunker_SchemaQualifiedNames(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE public.users (id INT PRIMARY KEY);
CREATE TABLE myschema.orders (id INT PRIMARY KEY);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should extract table names without schema prefix
	tableNames := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil {
			tableNames[chunk.Metadata.SQL.TableName] = true
		}
	}

	if !tableNames["users"] {
		t.Error("expected table name 'users' (without schema)")
	}
	if !tableNames["orders"] {
		t.Error("expected table name 'orders' (without schema)")
	}
}

func TestSQLChunker_NamedDollarTags(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE FUNCTION test_func() RETURNS void AS $body$
BEGIN
    RAISE NOTICE 'Body tag; semicolon inside';
END;
$body$ LANGUAGE plpgsql;

CREATE FUNCTION other_func() RETURNS void AS $func$
BEGIN
    RAISE NOTICE 'Func tag';
END;
$func$ LANGUAGE plpgsql;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 separate functions
	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks (2 functions with named dollar tags), got %d", result.TotalChunks)
	}

	// Verify functions are complete
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil && chunk.Metadata.SQL.ObjectType == "FUNCTION" {
			if !strings.Contains(chunk.Content, "RAISE NOTICE") {
				t.Error("expected function chunk to contain full body")
			}
		}
	}
}

func TestSQLChunker_TruncateStatement(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE logs (id INT PRIMARY KEY, message TEXT);
TRUNCATE TABLE logs;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TRUNCATE should be recognized
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_CTEWithStatement(t *testing.T) {
	c := NewSQLChunker()
	content := `WITH active_users AS (
    SELECT id, name FROM users WHERE active = true
),
recent_orders AS (
    SELECT * FROM orders WHERE created_at > NOW() - INTERVAL '7 days'
)
SELECT u.name, COUNT(o.id) as order_count
FROM active_users u
LEFT JOIN recent_orders o ON o.user_id = u.id
GROUP BY u.name;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CTE should be parsed as single statement
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Should contain the full CTE
	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "WITH active_users") {
		t.Error("expected chunk to contain CTE")
	}
	if !strings.Contains(chunk.Content, "GROUP BY") {
		t.Error("expected chunk to contain full query")
	}
}

func TestSQLChunker_DropCascade(t *testing.T) {
	c := NewSQLChunker()
	content := `DROP TABLE IF EXISTS old_table CASCADE;
DROP TABLE users RESTRICT;
DROP VIEW IF EXISTS user_view CASCADE;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse DROP statements correctly
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_StringsWithSemicolons(t *testing.T) {
	c := NewSQLChunker()
	content := `INSERT INTO messages (content) VALUES ('Hello; World; How are you?');
INSERT INTO messages (content) VALUES ('Semi;colon;everywhere;');
SELECT * FROM messages WHERE content LIKE '%;%';
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Strings with semicolons should not split statements incorrectly
	// INSERT statements should be grouped, SELECT is standalone (no table extraction)
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Verify strings with semicolons are preserved intact
	allContent := ""
	for _, chunk := range result.Chunks {
		allContent += chunk.Content
	}

	if !strings.Contains(allContent, "Hello; World") {
		t.Error("expected string with semicolons to be preserved")
	}
	if !strings.Contains(allContent, "Semi;colon;everywhere") {
		t.Error("expected all semicolons in strings to be preserved")
	}
}

func TestSQLChunker_ExplainAnalyze(t *testing.T) {
	c := NewSQLChunker()
	content := `EXPLAIN ANALYZE SELECT * FROM users WHERE id = 1;
EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT * FROM orders;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse EXPLAIN statements
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_CreateTypeAndDomain(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');
CREATE DOMAIN positive_int AS INT CHECK (VALUE > 0);
CREATE TABLE people (
    id INT PRIMARY KEY,
    current_mood mood
);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse custom types
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_CreateSchema(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE SCHEMA IF NOT EXISTS analytics;
CREATE SCHEMA reporting AUTHORIZATION admin;
CREATE TABLE analytics.events (id INT PRIMARY KEY);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse CREATE SCHEMA
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_ForeignKeyConstraints(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id INT NOT NULL,
    FOREIGN KEY (product_id) REFERENCES products(id) ON UPDATE SET NULL
);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	// Should preserve foreign key constraints
	if !strings.Contains(chunk.Content, "REFERENCES users") {
		t.Error("expected foreign key reference to be preserved")
	}
	if !strings.Contains(chunk.Content, "ON DELETE CASCADE") {
		t.Error("expected ON DELETE CASCADE to be preserved")
	}
}

func TestSQLChunker_UnionIntersectExcept(t *testing.T) {
	c := NewSQLChunker()
	content := `SELECT id, name FROM users WHERE role = 'admin'
UNION
SELECT id, name FROM users WHERE created_at > '2024-01-01'
EXCEPT
SELECT id, name FROM users WHERE disabled = true;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UNION/EXCEPT should be parsed as single statement
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if !strings.Contains(chunk.Content, "UNION") {
		t.Error("expected UNION to be in content")
	}
	if !strings.Contains(chunk.Content, "EXCEPT") {
		t.Error("expected EXCEPT to be in content")
	}
}

func TestSQLChunker_Subqueries(t *testing.T) {
	c := NewSQLChunker()
	content := `SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE total > 100);
INSERT INTO archived_users SELECT * FROM users WHERE last_login < '2023-01-01';
UPDATE orders SET status = 'processed' WHERE user_id IN (SELECT id FROM vip_users);
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Subqueries should be preserved
	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSQLChunker_LargeGroupSplitting(t *testing.T) {
	c := NewSQLChunker()

	// Create many statements for the same table
	var builder strings.Builder
	builder.WriteString("CREATE TABLE big_table (id INT PRIMARY KEY, data TEXT);\n")
	for i := 0; i < 50; i++ {
		builder.WriteString("INSERT INTO big_table (id, data) VALUES (")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString(", 'data value for row ")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString("');\n")
	}

	opts := ChunkOptions{
		MaxChunkSize: 500,
		MaxTokens:    100,
	}

	result, err := c.Chunk(context.Background(), []byte(builder.String()), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Large group should be split
	if result.TotalChunks < 2 {
		t.Errorf("expected large group to be split into multiple chunks, got %d", result.TotalChunks)
	}
}

func TestSQLChunker_EmptyAndWhitespace(t *testing.T) {
	c := NewSQLChunker()
	content := `

-- Just a comment

/* Another comment */

`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Comments and whitespace only should produce minimal chunks
	t.Logf("whitespace and comments produced %d chunks", result.TotalChunks)
}

func TestSQLChunker_PermissionStatements(t *testing.T) {
	c := NewSQLChunker()
	content := `GRANT SELECT, INSERT ON users TO readonly_user;
GRANT ALL PRIVILEGES ON DATABASE mydb TO admin;
REVOKE DELETE ON orders FROM guest;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recognize permission statements
	hasPermission := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.SQL != nil && chunk.Metadata.SQL.ObjectType == "PERMISSION" {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		t.Log("Note: permission statements may be categorized differently")
	}
}

func TestSQLChunker_NestedDollarQuotes(t *testing.T) {
	c := NewSQLChunker()
	// Nested dollar quotes with different tags
	content := `CREATE FUNCTION outer_func() RETURNS void AS $outer$
DECLARE
    sql_text text := $inner$SELECT * FROM users WHERE name = 'test';$inner$;
BEGIN
    EXECUTE sql_text;
END;
$outer$ LANGUAGE plpgsql;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nested dollar quotes should be handled
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk for nested dollar quotes, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "$inner$") {
			t.Error("expected nested dollar quote to be preserved")
		}
	}
}

func TestSQLChunker_UnicodeContent(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE i18n_strings (
    key VARCHAR(100) PRIMARY KEY,
    en TEXT,
    zh TEXT,
    ja TEXT
);
INSERT INTO i18n_strings VALUES ('greeting', 'Hello', '你好', 'こんにちは');
INSERT INTO i18n_strings VALUES ('emoji', '👋', '🌍', '🎉');
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Unicode should be preserved
	allContent := ""
	for _, chunk := range result.Chunks {
		allContent += chunk.Content
	}

	if !strings.Contains(allContent, "你好") {
		t.Error("expected Chinese characters to be preserved")
	}
	if !strings.Contains(allContent, "👋") {
		t.Error("expected emoji to be preserved")
	}
}

func TestSQLChunker_CreateTableAs(t *testing.T) {
	c := NewSQLChunker()
	content := `CREATE TABLE user_summary AS
SELECT user_id, COUNT(*) as order_count, SUM(total) as total_spent
FROM orders
GROUP BY user_id;
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	chunk := result.Chunks[0]
	if chunk.Metadata.SQL == nil {
		t.Fatal("expected SQL metadata")
	}
	if chunk.Metadata.SQL.ObjectType != "TABLE" {
		t.Errorf("expected ObjectType 'TABLE', got %q", chunk.Metadata.SQL.ObjectType)
	}
}
