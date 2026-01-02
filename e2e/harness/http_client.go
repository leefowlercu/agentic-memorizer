package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient provides a test client for the daemon's HTTP API
type HTTPClient struct {
	host   string
	port   int
	client *http.Client
}

// NewHTTPClient creates a new HTTP test client
func NewHTTPClient(host string, port int) *HTTPClient {
	return &HTTPClient{
		host: host,
		port: port,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SetPort updates the HTTP client's port
func (c *HTTPClient) SetPort(port int) {
	c.port = port
}

// baseURL returns the base URL for API requests
func (c *HTTPClient) baseURL() string {
	return fmt.Sprintf("http://%s:%d", c.host, c.port)
}

// Health checks the daemon health endpoint
func (c *HTTPClient) Health() (map[string]any, error) {
	resp, err := c.client.Get(c.baseURL() + "/health")
	if err != nil {
		return nil, fmt.Errorf("health request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("health check failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode health response; %w", err)
	}

	return health, nil
}

// WaitForHealthy polls the health endpoint until it responds or times out
func (c *HTTPClient) WaitForHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		_, err := c.Health()
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not become healthy within %v", timeout)
}

// TriggerRebuild triggers a full index rebuild
func (c *HTTPClient) TriggerRebuild(force bool) error {
	url := c.baseURL() + "/api/v1/rebuild"
	if force {
		url += "?force=true"
	}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create rebuild request; %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("rebuild request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rebuild failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// SearchFiles performs a semantic search using the unified files endpoint
func (c *HTTPClient) SearchFiles(query string, maxResults int) (any, error) {
	url := fmt.Sprintf("%s/api/v1/files?q=%s&limit=%d", c.baseURL(), query, maxResults)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("search request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var result any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search response; %w", err)
	}

	return result, nil
}

// GetFileMetadata retrieves metadata for a specific file
func (c *HTTPClient) GetFileMetadata(path string) (any, error) {
	url := fmt.Sprintf("%s/api/v1/files/%s", c.baseURL(), path)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("metadata request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("metadata retrieval failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var metadata any
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata response; %w", err)
	}

	return metadata, nil
}

// ListRecentFiles lists recently modified files using the unified files endpoint
func (c *HTTPClient) ListRecentFiles(days, limit int) (any, error) {
	url := fmt.Sprintf("%s/api/v1/files?days=%d&limit=%d", c.baseURL(), days, limit)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("recent files request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recent files retrieval failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var files any
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode recent files response; %w", err)
	}

	return files, nil
}

// GetRelatedFiles finds files related to a given file using the file metadata endpoint
func (c *HTTPClient) GetRelatedFiles(path string, limit int) (any, error) {
	url := fmt.Sprintf("%s/api/v1/files/%s?related_limit=%d", c.baseURL(), path, limit)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("related files request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("related files retrieval failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var files any
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode related files response; %w", err)
	}

	return files, nil
}

// SearchEntities searches for files mentioning a specific entity using the unified files endpoint
func (c *HTTPClient) SearchEntities(entity string, maxResults int) (any, error) {
	url := fmt.Sprintf("%s/api/v1/files?entity=%s&limit=%d", c.baseURL(), entity, maxResults)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("entity search request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("entity search failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var result any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode entity search response; %w", err)
	}

	return result, nil
}

// GetIndex retrieves the full index from the graph
func (c *HTTPClient) GetIndex() (any, error) {
	url := fmt.Sprintf("%s/api/v1/files/index", c.baseURL())

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("index request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("index retrieval failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var index any
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to decode index response; %w", err)
	}

	return index, nil
}

// GetIndexFiles retrieves the files array from the index response
// The API returns {index: {files: [...]}} so this helper unwraps it
func (c *HTTPClient) GetIndexFiles() ([]map[string]any, error) {
	index, err := c.GetIndex()
	if err != nil {
		return nil, err
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected index format; got %T", index)
	}

	innerIndex, ok := indexMap["index"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("index missing 'index' object")
	}

	filesAny, ok := innerIndex["files"].([]any)
	if !ok {
		return nil, fmt.Errorf("index missing 'files' array")
	}

	files := make([]map[string]any, 0, len(filesAny))
	for _, f := range filesAny {
		if fileMap, ok := f.(map[string]any); ok {
			files = append(files, fileMap)
		}
	}

	return files, nil
}

// GetFactsIndex retrieves all facts from the graph
func (c *HTTPClient) GetFactsIndex() (map[string]any, error) {
	url := fmt.Sprintf("%s/api/v1/facts/index", c.baseURL())

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("facts index request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("facts index retrieval failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode facts index response; %w", err)
	}

	return result, nil
}

// GetFact retrieves a specific fact by ID
func (c *HTTPClient) GetFact(id string) (map[string]any, error) {
	url := fmt.Sprintf("%s/api/v1/facts/%s", c.baseURL(), id)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fact request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("fact not found; status=404")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fact retrieval failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode fact response; %w", err)
	}

	return result, nil
}

// QueryFiles performs a unified files query with optional filters
func (c *HTTPClient) QueryFiles(query, category, entity, tag, topic string, days, limit int) (map[string]any, error) {
	url := fmt.Sprintf("%s/api/v1/files?limit=%d", c.baseURL(), limit)
	if query != "" {
		url += fmt.Sprintf("&q=%s", query)
	}
	if category != "" {
		url += fmt.Sprintf("&category=%s", category)
	}
	if entity != "" {
		url += fmt.Sprintf("&entity=%s", entity)
	}
	if tag != "" {
		url += fmt.Sprintf("&tag=%s", tag)
	}
	if topic != "" {
		url += fmt.Sprintf("&topic=%s", topic)
	}
	if days > 0 {
		url += fmt.Sprintf("&days=%d", days)
	}

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("files query request failed; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("files query failed; status=%d; body=%s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode files query response; %w", err)
	}

	return result, nil
}
