package metrics

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsProvider is an interface for components that provide metrics.
type MetricsProvider interface {
	// CollectMetrics collects current metrics from the component.
	CollectMetrics(ctx context.Context) error
}

// Collector manages metric collection from various components.
type Collector struct {
	mu        sync.RWMutex
	providers map[string]MetricsProvider
	interval  time.Duration
	stopCh    chan struct{}
	running   bool
}

// NewCollector creates a new metrics collector.
func NewCollector(interval time.Duration) *Collector {
	return &Collector{
		providers: make(map[string]MetricsProvider),
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Register adds a metrics provider to the collector.
func (c *Collector) Register(name string, provider MetricsProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers[name] = provider
}

// Unregister removes a metrics provider from the collector.
func (c *Collector) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.providers, name)
}

// Start begins periodic metric collection.
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	// Set daemon start time
	DaemonStartTime.Set(float64(time.Now().Unix()))

	// Set daemon info
	DaemonInfo.WithLabelValues("1.0.0", runtime.Version()).Set(1)

	// Initial collection
	c.collect(ctx)

	// Start periodic collection
	go c.run(ctx)

	return nil
}

// Stop halts periodic metric collection.
func (c *Collector) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	close(c.stopCh)
	c.running = false
	return nil
}

// run is the main collection loop.
func (c *Collector) run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

// collect gathers metrics from all registered providers.
func (c *Collector) collect(ctx context.Context) {
	c.mu.RLock()
	providers := make(map[string]MetricsProvider, len(c.providers))
	for k, v := range c.providers {
		providers[k] = v
	}
	c.mu.RUnlock()

	for name, provider := range providers {
		if err := provider.CollectMetrics(ctx); err != nil {
			ComponentStatus.WithLabelValues(name).Set(0)
		} else {
			ComponentStatus.WithLabelValues(name).Set(1)
		}
	}
}

// Handler returns the Prometheus HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// HandlerFor returns a handler for a specific registry.
func HandlerFor(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordAnalysis records an analysis operation.
func RecordAnalysis(analysisType string, duration time.Duration, err error) {
	AnalysisTotal.WithLabelValues(analysisType).Inc()
	AnalysisDuration.WithLabelValues(analysisType).Observe(duration.Seconds())
	if err != nil {
		AnalysisErrorsTotal.WithLabelValues(analysisType, "").Inc()
	}
}

// RecordProviderRequest records a provider API request.
func RecordProviderRequest(provider, operation string, duration time.Duration, inputTokens, outputTokens int, err error) {
	ProviderRequestsTotal.WithLabelValues(provider, operation).Inc()
	ProviderDuration.WithLabelValues(provider, operation).Observe(duration.Seconds())

	if inputTokens > 0 {
		ProviderTokensTotal.WithLabelValues(provider, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		ProviderTokensTotal.WithLabelValues(provider, "output").Add(float64(outputTokens))
	}

	if err != nil {
		ProviderErrorsTotal.WithLabelValues(provider, operation).Inc()
	}
}

// RecordCacheAccess records a cache access.
func RecordCacheAccess(cacheType string, hit bool) {
	if hit {
		CacheHitsTotal.WithLabelValues(cacheType).Inc()
	} else {
		CacheMissesTotal.WithLabelValues(cacheType).Inc()
	}
}

// RecordGraphOperation records a graph operation.
func RecordGraphOperation(operation string, duration time.Duration, err error) {
	GraphOperationsTotal.WithLabelValues(operation).Inc()
	GraphOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if err != nil {
		GraphOperationErrorsTotal.WithLabelValues(operation).Inc()
	}
}

// RecordWatcherEvent records a filesystem event.
func RecordWatcherEvent(eventType string) {
	WatcherEventsTotal.WithLabelValues(eventType).Inc()
}

// RecordMCPRequest records an MCP request.
func RecordMCPRequest(method string) {
	MCPRequestsTotal.WithLabelValues(method).Inc()
}

// UpdateQueueMetrics updates the analysis queue metrics.
func UpdateQueueMetrics(pending, inProgress int) {
	QueuePending.Set(float64(pending))
	QueueInProgress.Set(float64(inProgress))
}

// UpdateGraphMetrics updates the graph state metrics.
func UpdateGraphMetrics(files, directories, chunks int) {
	FilesTotal.Set(float64(files))
	DirectoriesTotal.Set(float64(directories))
	ChunksTotal.Set(float64(chunks))
}

// UpdateWatcherMetrics updates the watcher metrics.
func UpdateWatcherMetrics(pathCount int) {
	WatcherPathsTotal.Set(float64(pathCount))
}

// UpdateMCPMetrics updates the MCP metrics.
func UpdateMCPMetrics(subscriptions int) {
	MCPSubscriptionsTotal.Set(float64(subscriptions))
}
