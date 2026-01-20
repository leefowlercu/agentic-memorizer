package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultMinBackoff = time.Second
	defaultMaxBackoff = 30 * time.Second
)

// ComponentSupervisor manages runtime supervision of persistent components.
// It handles component startup, restart on failure with exponential backoff,
// and health status updates.
type ComponentSupervisor struct {
	componentCancels map[string]context.CancelFunc
	healthUpdater    HealthUpdater
	logger           *slog.Logger
	minBackoff       time.Duration
	maxBackoff       time.Duration
	mu               sync.Mutex
}

// SupervisorOption configures ComponentSupervisor.
type SupervisorOption func(*ComponentSupervisor)

// WithSupervisorLogger sets the logger for supervision.
func WithSupervisorLogger(l *slog.Logger) SupervisorOption {
	return func(s *ComponentSupervisor) {
		s.logger = l
	}
}

// WithBackoff sets the min and max backoff durations.
func WithBackoff(min, max time.Duration) SupervisorOption {
	return func(s *ComponentSupervisor) {
		s.minBackoff = min
		s.maxBackoff = max
	}
}

// NewComponentSupervisor creates a new supervisor.
func NewComponentSupervisor(healthUpdater HealthUpdater, opts ...SupervisorOption) *ComponentSupervisor {
	s := &ComponentSupervisor{
		componentCancels: make(map[string]context.CancelFunc),
		healthUpdater:    healthUpdater,
		logger:           slog.Default(),
		minBackoff:       defaultMinBackoff,
		maxBackoff:       defaultMaxBackoff,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Supervise starts supervision for a named component with backoff restart.
// Parameters:
//   - ctx: parent context for cancellation
//   - name: component name for logging/health
//   - def: component definition (for RestartPolicy, Criticality)
//   - startFn: function to start the component (blocks during run)
//   - fatalCh: optional channel signaling runtime fatal errors
func (s *ComponentSupervisor) Supervise(
	ctx context.Context,
	name string,
	def ComponentDefinition,
	startFn func(context.Context) error,
	fatalCh <-chan error,
) {
	startCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.componentCancels[name] = cancel
	s.mu.Unlock()

	backoff := s.minBackoff

	go func() {
		for {
			err := startFn(startCtx)
			now := time.Now()

			if err != nil {
				s.logger.Warn("component start/run failed",
					"component", name,
					"error", err,
				)
				if s.healthUpdater != nil {
					s.healthUpdater.UpdateComponentHealth(map[string]ComponentHealth{
						name: {
							Status:      ComponentStatusFailed,
							Error:       err.Error(),
							LastChecked: now,
						},
					})
				}

				if def.RestartPolicy == RestartNever {
					if def.Criticality == CriticalityFatal {
						s.logger.Error("fatal component failed and will not restart",
							"component", name,
							"error", err,
						)
					}
					return
				}
			} else {
				if s.healthUpdater != nil {
					s.healthUpdater.UpdateComponentHealth(map[string]ComponentHealth{
						name: {
							Status:      ComponentStatusRunning,
							LastChecked: now,
							LastSuccess: now,
						},
					})
				}

				if def.RestartPolicy == RestartNever {
					return
				}

				// Wait for fatal error or cancellation before restart
				if fatalCh != nil {
					select {
					case <-startCtx.Done():
						return
					case fatalErr := <-fatalCh:
						if fatalErr != nil {
							s.logger.Warn("component runtime error; restarting",
								"component", name,
								"error", fatalErr,
							)
							err = fatalErr
						}
					}
				} else {
					// No fatal channel; stay running until context cancel
					<-startCtx.Done()
					return
				}
			}

			select {
			case <-startCtx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > s.maxBackoff {
				backoff = s.maxBackoff
			}
		}
	}()
}

// Cancel cancels supervision for a specific component.
func (s *ComponentSupervisor) Cancel(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.componentCancels[name]; ok {
		cancel()
		delete(s.componentCancels, name)
	}
}

// CancelAll cancels all supervised components.
func (s *ComponentSupervisor) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, cancel := range s.componentCancels {
		s.logger.Debug("canceling component", "component", name)
		cancel()
	}
	s.componentCancels = make(map[string]context.CancelFunc)
}

// SupervisedCount returns the number of currently supervised components.
func (s *ComponentSupervisor) SupervisedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.componentCancels)
}
