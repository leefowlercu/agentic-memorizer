package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// RememberService handles remembering and forgetting paths via the registry and event bus.
type RememberService struct {
	registry registry.Registry
	bus      events.Bus
	defaults config.DefaultsConfig
	logger   *slog.Logger
}

// RememberServiceOption configures RememberService.
type RememberServiceOption func(*RememberService)

// WithLogger sets the logger for RememberService.
func WithLogger(logger *slog.Logger) RememberServiceOption {
	return func(s *RememberService) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// NewRememberService creates a RememberService.
func NewRememberService(reg registry.Registry, bus events.Bus, defaults config.DefaultsConfig, opts ...RememberServiceOption) *RememberService {
	s := &RememberService{
		registry: reg,
		bus:      bus,
		defaults: defaults,
		logger:   slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Remember adds or updates a remembered path.
func (s *RememberService) Remember(ctx context.Context, req RememberRequest) (*RememberResponse, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	absPath, err := resolvePath(req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path; %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", absPath)
		}
		return nil, fmt.Errorf("failed to access path; %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	existing, err := s.registry.GetPath(ctx, absPath)
	if err == nil && existing != nil {
		if req.Patch == nil || req.Patch.IsEmpty() {
			return nil, fmt.Errorf("path is already remembered: %s\nUse modification flags (--add-*, --set-*, --skip-hidden) to update configuration", absPath)
		}

		updated := registry.ApplyPathConfigPatch(existing.Config, req.Patch)
		if err := s.registry.UpdatePathConfig(ctx, absPath, updated); err != nil {
			return nil, fmt.Errorf("failed to update path config; %w", err)
		}

		s.publishRememberedPathEvent(ctx, events.NewRememberedPathUpdated(absPath))
		return &RememberResponse{
			Status: RememberStatusUpdated,
			Path:   absPath,
		}, nil
	}
	if err != nil && err != registry.ErrPathNotFound {
		return nil, fmt.Errorf("failed to check path; %w", err)
	}

	base := defaultPathConfig(s.defaults)
	config := registry.ApplyPathConfigPatch(base, req.Patch)
	if err := s.registry.AddPath(ctx, absPath, config); err != nil {
		if err == registry.ErrPathExists {
			return nil, fmt.Errorf("path is already remembered: %s", absPath)
		}
		return nil, fmt.Errorf("failed to remember path; %w", err)
	}

	s.publishRememberedPathEvent(ctx, events.NewRememberedPathAdded(absPath))
	return &RememberResponse{
		Status: RememberStatusAdded,
		Path:   absPath,
	}, nil
}

// Forget removes a remembered path and optionally keeps its data.
func (s *RememberService) Forget(ctx context.Context, req ForgetRequest) (*ForgetResponse, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	absPath, err := resolvePath(req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path; %w", err)
	}

	_, err = s.registry.GetPath(ctx, absPath)
	if err != nil {
		if err == registry.ErrPathNotFound {
			return nil, fmt.Errorf("path is not remembered: %s", absPath)
		}
		return nil, fmt.Errorf("failed to check path; %w", err)
	}

	if !req.KeepData {
		if err := s.registry.DeleteFileStatesForPath(ctx, absPath); err != nil {
			return nil, fmt.Errorf("failed to delete file states; %w", err)
		}
	}

	if err := s.registry.RemovePath(ctx, absPath); err != nil {
		return nil, fmt.Errorf("failed to forget path; %w", err)
	}

	s.publishRememberedPathEvent(ctx, events.NewRememberedPathRemoved(absPath, "forgotten", req.KeepData))
	return &ForgetResponse{
		Status:   ForgetStatusForgotten,
		Path:     absPath,
		KeepData: req.KeepData,
	}, nil
}

func (s *RememberService) publishRememberedPathEvent(ctx context.Context, event events.Event) {
	if s.bus == nil {
		s.logger.Warn("bus is nil, cannot publish event", "event_type", event.Type)
		return
	}

	s.logger.Info("publishing remembered path event", "event_type", event.Type, "path", eventPayloadPath(event))
	if err := s.bus.Publish(ctx, event); err != nil {
		s.logger.Warn("failed to publish remembered path event",
			"event_type", event.Type,
			"path", eventPayloadPath(event),
			"error", err,
		)
	}
}

func defaultPathConfig(defaults config.DefaultsConfig) *registry.PathConfig {
	return &registry.PathConfig{
		SkipHidden:         defaults.Skip.Hidden,
		SkipExtensions:     append([]string{}, defaults.Skip.Extensions...),
		SkipDirectories:    append([]string{}, defaults.Skip.Directories...),
		SkipFiles:          append([]string{}, defaults.Skip.Files...),
		IncludeExtensions:  append([]string{}, defaults.Include.Extensions...),
		IncludeDirectories: append([]string{}, defaults.Include.Directories...),
		IncludeFiles:       append([]string{}, defaults.Include.Files...),
	}
}

func resolvePath(path string) (string, error) {
	expanded := config.ExpandPath(path)
	if expanded == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	return filepath.Clean(absPath), nil
}

func eventPayloadPath(event events.Event) string {
	switch payload := event.Payload.(type) {
	case *events.RememberedPathEvent:
		return payload.Path
	case *events.RememberedPathRemovedEvent:
		return payload.Path
	default:
		return ""
	}
}
