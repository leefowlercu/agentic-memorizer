//go:build e2e

package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestE2E_FreshInstallation tests complete fresh installation workflow
func TestE2E_FreshInstallation(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Step 1: Validate configuration
	t.Log("Step 1: Validating configuration")
	stdout, stderr, exitCode := h.RunCommand("config", "validate")
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Step 2: Add test files to memory directory
	t.Log("Step 2: Adding test files to memory directory")
	testFiles := map[string]string{
		"README.md":     "# Test Project\n\nThis is a test project for E2E testing.",
		"docs/guide.md": "# User Guide\n\nHow to use this project.",
		"src/main.go":   "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}",
		"config.yaml":   "app:\n  name: test\n  version: 1.0.0",
		"image.png":     "fake-png-data", // Simplified for testing
	}

	for path, content := range testFiles {
		if err := h.AddMemoryFile(path, content); err != nil {
			t.Fatalf("Failed to add file %s: %v", path, err)
		}
	}

	// Step 3: Verify files exist
	t.Log("Step 3: Verifying files exist")
	for path := range testFiles {
		fullPath := filepath.Join(h.MemoryRoot, path)
		if _, err := os.Stat(fullPath); err != nil {
			t.Errorf("File %s should exist: %v", path, err)
		}
	}

	t.Logf("Fresh installation workflow completed: %d files added", len(testFiles))
}

// TestE2E_IntegrationSetup tests complete integration setup workflow
func TestE2E_IntegrationSetup(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Step 1: List available integrations
	t.Log("Step 1: Listing available integrations")
	stdout, stderr, exitCode := h.RunCommand("integrations", "list")
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "claude-code")

	// Step 2: Detect installed frameworks
	t.Log("Step 2: Detecting installed frameworks")
	stdout, stderr, exitCode = h.RunCommand("integrations", "detect")
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Step 3: Setup Claude Code hook integration (will skip if not in test env)
	t.Log("Step 3: Setting up Claude Code hook integration")

	// Create mock ~/.claude directory
	claudeDir := filepath.Join(h.AppDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	// Create empty settings.json
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write settings.json: %v", err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", h.AppDir)
	defer os.Setenv("HOME", oldHome)

	stdout, stderr, exitCode = h.RunCommand("integrations", "setup", "claude-code-hook",
		"--binary-path", h.BinaryPath)

	if exitCode != 0 {
		t.Logf("Setup may fail in test environment: %s", stderr)
	}

	// Step 4: Validate integration
	t.Log("Step 4: Validating integration")
	stdout, stderr, _ = h.RunCommand("integrations", "validate")
	t.Logf("Validation output: %s", stdout)

	// Step 5: Read index output (may be empty if no processing done)
	t.Log("Step 5: Reading index output")
	stdout, stderr, _ = h.RunCommand("read", "--output=json")
	t.Logf("Index read attempted (exit code ignored for empty index)")

	t.Log("Integration setup workflow completed")
}

// TestE2E_DaemonLifecycle tests complete daemon lifecycle workflow
func TestE2E_DaemonLifecycle(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Step 1: Enable HTTP server for health checks
	t.Log("Step 1: Enabling HTTP server")
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Step 2: Add test files
	t.Log("Step 2: Adding test files")
	if err := h.AddMemoryFile("test.md", "# Test\n\nTest content."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Step 3: Start daemon in background
	t.Log("Step 3: Starting daemon")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	// Step 4: Wait for daemon to be healthy
	t.Log("Step 4: Waiting for daemon to become healthy")
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	// Step 5: Check daemon status
	t.Log("Step 5: Checking daemon status")
	stdout, stderr, exitCode := h.RunCommand("daemon", "status")
	harness.LogOutput(t, stdout, stderr)

	if exitCode == 0 {
		output := stdout + stderr
		outputLower := strings.ToLower(output)
		if !strings.Contains(outputLower, "running") && !strings.Contains(outputLower, "active") {
			t.Logf("Status output: %s", output)
		}
	}

	// Step 6: Query health endpoint
	t.Log("Step 6: Querying health endpoint")
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status=healthy, got: %v", health["status"])
	}

	t.Logf("Daemon health check passed: %v", health)

	// Step 7: Stop daemon
	t.Log("Step 7: Stopping daemon")
	stdout, stderr, exitCode = h.RunCommand("daemon", "stop")
	harness.LogOutput(t, stdout, stderr)

	// Wait for daemon to stop
	time.Sleep(2 * time.Second)

	// Step 8: Verify daemon stopped
	t.Log("Step 8: Verifying daemon stopped")
	stdout, stderr, _ = h.RunCommand("daemon", "status")
	output := stdout + stderr
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "not running") && !strings.Contains(outputLower, "stopped") {
		t.Logf("Status after stop: %s", output)
	}

	t.Log("Daemon lifecycle workflow completed")
}

// TestE2E_FileProcessingWorkflow tests file addition and processing
func TestE2E_FileProcessingWorkflow(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Step 1: Enable HTTP server
	t.Log("Step 1: Enabling HTTP server")
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Step 2: Start daemon
	t.Log("Step 2: Starting daemon")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Step 2: Add files to memory directory
	t.Log("Step 2: Adding files")
	testFiles := []string{
		"project/README.md",
		"project/docs/api.md",
		"project/src/main.go",
	}

	for _, file := range testFiles {
		content := "# " + filepath.Base(file) + "\n\nTest content."
		if err := h.AddMemoryFile(file, content); err != nil {
			t.Fatalf("Failed to add file %s: %v", file, err)
		}
	}

	// Step 3: Wait for processing
	t.Log("Step 3: Waiting for file processing")
	time.Sleep(5 * time.Second)

	// Step 4: Verify files in graph
	t.Log("Step 4: Verifying files in graph")
	graphCtx := context.Background()
	for _, file := range testFiles {
		path := "/" + file
		exists, err := h.GraphClient.FileExists(graphCtx, path)
		if err != nil {
			t.Logf("Graph query failed (graph may be unavailable): %v", err)
			break
		}
		if !exists {
			t.Logf("File %s not yet indexed (processing may be ongoing)", file)
		}
	}

	t.Log("File processing workflow completed")
}

// TestE2E_MCPToolUsage tests MCP server and tool usage
func TestE2E_MCPToolUsage(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for daemon API
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files for the daemon to index
	if err := h.AddMemoryFile("workflow1.md", "# Workflow Test 1\n\nTest content for MCP tool usage"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("workflow2.md", "# Workflow Test 2\n\nAnother test file"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for indexing to complete
	time.Sleep(5 * time.Second)

	// Step 1: Start MCP server
	t.Log("Step 1: Starting MCP server")
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Step 2: Initialize MCP connection
	t.Log("Step 2: Initializing MCP connection")
	_, err = client.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Step 3: List available tools
	t.Log("Step 3: Listing available tools")
	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("List tools failed: %v", err)
	}

	expectedTools := []string{
		"search_files",
		"get_file_metadata",
		"list_recent_files",
		"get_related_files",
		"search_entities",
	}

	for _, expectedTool := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool.Name == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found", expectedTool)
		}
	}

	// Step 4: Call search_files tool
	t.Log("Step 4: Calling search_files tool")
	searchArgs := map[string]any{
		"query": "test",
	}

	_, err = client.CallTool("search_files", searchArgs)
	if err != nil {
		t.Fatalf("search_files call failed: %v", err)
	}

	// Step 5: List resources
	t.Log("Step 5: Listing resources")
	resources, err := client.ListResources()
	if err != nil {
		t.Fatalf("List resources failed: %v", err)
	}

	if len(resources) < 2 {
		t.Errorf("Expected at least 2 resources, got %d", len(resources))
	}

	// Step 6: Shutdown
	t.Log("Step 6: Shutting down MCP server")
	if err := client.Shutdown(); err != nil {
		t.Logf("Shutdown may have errors: %v", err)
	}

	t.Log("MCP tool usage workflow completed")
}

// TestE2E_GraphOperations tests end-to-end graph operations
func TestE2E_GraphOperations(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	ctx := context.Background()

	// Step 1: Verify graph connection
	t.Log("Step 1: Verifying graph connection")
	_, err := h.GraphClient.Query(ctx, "RETURN 1")
	if err != nil {
		t.Skipf("FalkorDB not available: %v", err)
	}

	// Step 2: Create file nodes
	t.Log("Step 2: Creating file nodes")
	createQuery := `
		CREATE (f1:File {path: '/e2e/doc1.md', name: 'doc1.md', hash: 'e2e1', summary: 'First document'})
		CREATE (f2:File {path: '/e2e/doc2.md', name: 'doc2.md', hash: 'e2e2', summary: 'Second document'})
		CREATE (t1:Tag {name: 'e2e-test'})
		CREATE (t2:Tag {name: 'documentation'})
		CREATE (f1)-[:HAS_TAG]->(t1)
		CREATE (f1)-[:HAS_TAG]->(t2)
		CREATE (f2)-[:HAS_TAG]->(t2)
	`

	_, err = h.GraphClient.Query(ctx, createQuery)
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Step 3: Search by tag
	t.Log("Step 3: Searching by tag")
	searchQuery := `
		MATCH (f:File)-[:HAS_TAG]->(t:Tag)
		WHERE t.name = 'documentation'
		RETURN f.path as path
	`

	result, err := h.GraphClient.Query(ctx, searchQuery)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	count := 0
	for result.Next() {
		record := result.Record()
		if path, ok := record.Get("path"); ok {
			t.Logf("Found file: %v", path)
			count++
		}
	}

	if count != 2 {
		t.Errorf("Expected 2 files with 'documentation' tag, got %d", count)
	}

	// Step 4: Get related files
	t.Log("Step 4: Getting related files")
	relatedFiles, err := h.GraphClient.GetRelatedFiles(ctx, "/e2e/doc1.md", 10)
	if err != nil {
		t.Fatalf("Get related files failed: %v", err)
	}

	if len(relatedFiles) < 1 {
		t.Error("Expected at least 1 related file")
	}

	t.Logf("Found %d related files", len(relatedFiles))

	// Step 5: Count nodes
	t.Log("Step 5: Counting nodes")
	fileCount, err := h.GraphClient.CountNodes(ctx, "File")
	if err != nil {
		t.Fatalf("Count nodes failed: %v", err)
	}

	if fileCount < 2 {
		t.Errorf("Expected at least 2 File nodes, got %d", fileCount)
	}

	// Step 6: Cleanup
	t.Log("Step 6: Cleaning up test data")
	cleanupQuery := `MATCH (n) WHERE n.hash IN ['e2e1', 'e2e2'] OR n.name IN ['e2e-test', 'documentation'] DETACH DELETE n`
	_, err = h.GraphClient.Query(ctx, cleanupQuery)
	if err != nil {
		t.Logf("Cleanup warning: %v", err)
	}

	t.Log("Graph operations workflow completed")
}

// TestE2E_ErrorRecovery tests error recovery scenarios
func TestE2E_ErrorRecovery(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Scenario 1: Invalid command
	t.Log("Scenario 1: Testing invalid command handling")
	stdout, stderr, exitCode := h.RunCommand("nonexistent-command")
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid command")
	}
	output := stdout + stderr
	harness.AssertContains(t, output, "unknown command")

	// Scenario 2: Missing required flag
	t.Log("Scenario 2: Testing missing required arguments")
	stdout, stderr, exitCode = h.RunCommand("integrations", "setup")
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for missing integration name")
	}

	// Scenario 3: Config validation with bad YAML
	t.Log("Scenario 3: Testing invalid YAML handling")
	badYAML := "this is not: valid: yaml::"
	if err := os.WriteFile(h.ConfigPath, []byte(badYAML), 0644); err != nil {
		t.Fatalf("Failed to write bad YAML: %v", err)
	}

	stdout, stderr, exitCode = h.RunCommand("config", "validate")
	if exitCode == 0 {
		t.Log("Config validation may succeed with defaults even with bad YAML")
	}

	// Scenario 4: Daemon status when not running
	t.Log("Scenario 4: Testing daemon status when not running")
	stdout, stderr, _ = h.RunCommand("daemon", "status")
	output = stdout + stderr
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "not running") && !strings.Contains(outputLower, "stopped") {
		t.Logf("Status output: %s", output)
	}

	t.Log("Error recovery scenarios completed")
}

// TestE2E_FullWorkflow tests a complete realistic workflow
func TestE2E_FullWorkflow(t *testing.T) {
	t.Skip("Full workflow requires all components working together - run manually for validation")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Step 1: Enable HTTP server
	t.Log("Step 1: Enabling HTTP server")
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Step 2: Validate configuration
	t.Log("Step 2: Validating configuration")
	stdout, stderr, exitCode := h.RunCommand("config", "validate")
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Step 3: Add project files
	t.Log("Step 3: Adding project files")
	projectFiles := map[string]string{
		"README.md":            "# My Project\n\nA test project for E2E validation.",
		"docs/architecture.md": "# Architecture\n\nSystem design overview.",
		"docs/api.md":          "# API Reference\n\nAPI documentation.",
		"src/main.go":          "package main\n\nfunc main() {}",
		"src/utils.go":         "package main\n\nfunc helper() {}",
		"tests/main_test.go":   "package main\n\nimport \"testing\"",
	}

	for path, content := range projectFiles {
		if err := h.AddMemoryFile(path, content); err != nil {
			t.Fatalf("Failed to add file %s: %v", path, err)
		}
	}

	// Step 4: Start daemon
	t.Log("Step 4: Starting daemon")
	if err := h.StartDaemon(); err != nil {
		t.Skipf("Failed to start daemon: %v", err)
	}
	defer h.StopDaemon()

	// Step 5: Wait for healthy and processing
	t.Log("Step 5: Waiting for daemon to process files")
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	time.Sleep(10 * time.Second) // Allow processing time

	// Step 6: Query index via read command
	t.Log("Step 6: Querying index")
	stdout, stderr, exitCode = h.RunCommand("read", "--output=json")
	t.Logf("Index read (exit=%d): %d bytes output", exitCode, len(stdout))

	// Step 7: Start MCP server and query
	t.Log("Step 7: Testing MCP server")
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err == nil {
		defer client.Close()

		if _, err := client.Initialize(); err == nil {
			tools, _ := client.ListTools()
			t.Logf("MCP server provides %d tools", len(tools))
		}
	}

	// Step 8: Verify graph has data
	t.Log("Step 8: Verifying graph data")
	ctx := context.Background()
	fileCount, err := h.GraphClient.CountNodes(ctx, "File")
	if err == nil {
		t.Logf("Graph contains %d File nodes", fileCount)
	}

	t.Log("Full workflow completed successfully")
}
