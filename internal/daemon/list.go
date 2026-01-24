package daemon

import (
	"context"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// ListEntry represents a remembered path with status and optional file count.
type ListEntry struct {
	Path       string               `json:"path"`
	Status     string               `json:"status"`
	FileCount  *int                 `json:"file_count,omitempty"`
	LastWalkAt *time.Time           `json:"last_walk_at,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
	Config     *registry.PathConfig `json:"config,omitempty"`
}

// ListResponse is the response payload for /list.
type ListResponse struct {
	Paths []ListEntry `json:"paths"`
}

// ListFunc handles list requests.
type ListFunc func(ctx context.Context) (*ListResponse, error)

// ListService provides list operations backed by the registry.
type ListService struct {
	registry registry.Registry
}

// NewListService creates a new ListService.
func NewListService(reg registry.Registry) *ListService {
	return &ListService{registry: reg}
}

// List returns remembered paths with health status and file counts when available.
func (s *ListService) List(ctx context.Context) (*ListResponse, error) {
	paths, err := s.registry.ListPaths(ctx)
	if err != nil {
		return nil, err
	}

	resp := &ListResponse{Paths: make([]ListEntry, 0, len(paths))}
	if len(paths) == 0 {
		return resp, nil
	}

	statuses, err := s.registry.CheckPathHealth(ctx)
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]string, len(statuses))
	for _, s := range statuses {
		statusMap[s.Path] = s.Status
	}

	for _, p := range paths {
		status := statusMap[p.Path]
		if status == "" {
			status = registry.PathStatusError
		}

		entry := ListEntry{
			Path:       p.Path,
			Status:     status,
			LastWalkAt: p.LastWalkAt,
			CreatedAt:  p.CreatedAt,
			UpdatedAt:  p.UpdatedAt,
			Config:     p.Config,
		}

		if status == registry.PathStatusOK {
			states, err := s.registry.ListFileStates(ctx, p.Path)
			if err == nil {
				count := len(states)
				entry.FileCount = &count
			}
		}

		resp.Paths = append(resp.Paths, entry)
	}

	return resp, nil
}
