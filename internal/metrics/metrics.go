// Package metrics provides Prometheus metrics for the memorizer daemon.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "memorizer"
)

// Graph metrics track the state of the knowledge graph.
var (
	// FilesTotal is the total number of files in the graph.
	FilesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "files_total",
		Help:      "Total number of files in the knowledge graph",
	})

	// DirectoriesTotal is the total number of remembered directories.
	DirectoriesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "directories_total",
		Help:      "Total number of remembered directories",
	})

	// ChunksTotal is the total number of chunks in the graph.
	ChunksTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "chunks_total",
		Help:      "Total number of chunks in the knowledge graph",
	})
)

// Queue metrics track the analysis queue state.
var (
	// QueuePending is the number of files pending analysis.
	QueuePending = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "queue_pending",
		Help:      "Number of files pending analysis",
	})

	// QueueInProgress is the number of files currently being analyzed.
	QueueInProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "queue_in_progress",
		Help:      "Number of files currently being analyzed",
	})
)

// Analysis metrics track file analysis operations.
var (
	// AnalysisTotal is the total number of files analyzed by type.
	AnalysisTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "analysis_total",
		Help:      "Total number of files analyzed",
	}, []string{"type"})

	// AnalysisErrorsTotal is the total number of analysis errors by type and provider.
	AnalysisErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "analysis_errors_total",
		Help:      "Total number of analysis errors",
	}, []string{"type", "provider"})

	// AnalysisDuration is a histogram of analysis duration in seconds.
	AnalysisDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "analysis_duration_seconds",
		Help:      "Duration of file analysis in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~102s
	}, []string{"type"})
)

// Cache metrics track cache operations.
var (
	// CacheHitsTotal is the total number of cache hits by cache type.
	CacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cache_hits_total",
		Help:      "Total number of cache hits",
	}, []string{"cache"})

	// CacheMissesTotal is the total number of cache misses by cache type.
	CacheMissesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cache_misses_total",
		Help:      "Total number of cache misses",
	}, []string{"cache"})
)

// Provider metrics track AI provider API usage.
var (
	// ProviderRequestsTotal is the total number of provider API requests.
	ProviderRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "provider_requests_total",
		Help:      "Total number of provider API requests",
	}, []string{"provider", "operation"})

	// ProviderErrorsTotal is the total number of provider API errors.
	ProviderErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "provider_errors_total",
		Help:      "Total number of provider API errors",
	}, []string{"provider", "operation"})

	// ProviderTokensTotal is the total number of tokens consumed.
	ProviderTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "provider_tokens_total",
		Help:      "Total number of tokens consumed",
	}, []string{"provider", "type"})

	// ProviderDuration is a histogram of provider request duration in seconds.
	ProviderDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "provider_duration_seconds",
		Help:      "Duration of provider API requests in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~102s
	}, []string{"provider", "operation"})
)

// Watcher metrics track filesystem monitoring.
var (
	// WatcherEventsTotal is the total number of filesystem events.
	WatcherEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "watcher_events_total",
		Help:      "Total number of filesystem events",
	}, []string{"type"})

	// WatcherPathsTotal is the total number of paths being watched.
	WatcherPathsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "watcher_paths_total",
		Help:      "Total number of paths being watched",
	})
)

// Graph operation metrics track database operations.
var (
	// GraphOperationsTotal is the total number of graph operations.
	GraphOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "graph_operations_total",
		Help:      "Total number of graph operations",
	}, []string{"operation"})

	// GraphOperationDuration is a histogram of graph operation duration.
	GraphOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "graph_operation_duration_seconds",
		Help:      "Duration of graph operations in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
	}, []string{"operation"})

	// GraphOperationErrorsTotal is the total number of graph operation errors.
	GraphOperationErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "graph_operation_errors_total",
		Help:      "Total number of graph operation errors",
	}, []string{"operation"})
)

// Walker metrics track directory scanning.
var (
	// WalkerFilesDiscovered is the total number of files discovered during walks.
	WalkerFilesDiscovered = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "walker_files_discovered_total",
		Help:      "Total number of files discovered during walks",
	})

	// WalkerFilesSkipped is the total number of files skipped during walks.
	WalkerFilesSkipped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "walker_files_skipped_total",
		Help:      "Total number of files skipped during walks",
	})

	// WalkerDuration is a histogram of walk duration in seconds.
	WalkerDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "walker_duration_seconds",
		Help:      "Duration of directory walks in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 12), // 0.1s to ~409s
	})
)

// MCP metrics track MCP server operations.
var (
	// MCPRequestsTotal is the total number of MCP requests.
	MCPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "mcp_requests_total",
		Help:      "Total number of MCP requests",
	}, []string{"method"})

	// MCPSubscriptionsTotal is the total number of active MCP subscriptions.
	MCPSubscriptionsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "mcp_subscriptions_total",
		Help:      "Total number of active MCP subscriptions",
	})
)

// Daemon metrics track daemon health and uptime.
var (
	// DaemonInfo provides daemon version and build information.
	DaemonInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "daemon_info",
		Help:      "Daemon version and build information",
	}, []string{"version", "go_version"})

	// DaemonStartTime is the unix timestamp when the daemon started.
	DaemonStartTime = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "daemon_start_time_seconds",
		Help:      "Unix timestamp when the daemon started",
	})

	// ComponentStatus tracks the health status of daemon components.
	ComponentStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "component_status",
		Help:      "Health status of daemon components (1=healthy, 0=unhealthy)",
	}, []string{"component"})
)
