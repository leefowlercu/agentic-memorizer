package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/export"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// ErrReadUnavailable indicates the graph is not ready to serve read requests.
var ErrReadUnavailable = errors.New("read not available")

// ReadRequest defines the payload for /read.
type ReadRequest struct {
	Format   string `json:"format"`
	Envelope string `json:"envelope"`
	MaxFiles int    `json:"max_files"`
}

// ReadResponse defines the response for /read.
type ReadResponse struct {
	Output string              `json:"output"`
	Stats  *export.ExportStats `json:"stats"`
}

// ReadFunc handles read requests.
type ReadFunc func(ctx context.Context, req ReadRequest) (*ReadResponse, error)

// ReadService handles graph export requests.
type ReadService struct {
	graph graph.Graph
}

// NewReadService creates a new ReadService.
func NewReadService(g graph.Graph) *ReadService {
	return &ReadService{graph: g}
}

// Read exports the knowledge graph with the given options.
func (s *ReadService) Read(ctx context.Context, req ReadRequest) (*ReadResponse, error) {
	if s.graph == nil || !s.graph.IsConnected() {
		return nil, ErrReadUnavailable
	}

	if req.MaxFiles < 0 {
		return nil, fmt.Errorf("max_files must be >= 0")
	}

	opts := export.DefaultExportOptions()
	if req.Format != "" {
		opts.Format = req.Format
	}
	if req.Envelope != "" {
		opts.Envelope = req.Envelope
	}
	if req.MaxFiles > 0 {
		opts.MaxFiles = req.MaxFiles
	}

	exporter := export.NewExporter(s.graph)
	output, stats, err := exporter.Export(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &ReadResponse{
		Output: string(output),
		Stats:  stats,
	}, nil
}
