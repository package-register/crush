package hook

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ManagerConfig holds hook manager configuration.
type ManagerConfig struct {
	// Enabled enables hook execution.
	Enabled bool
	// Async executes hooks asynchronously if true.
	Async bool
	// Timeout is the maximum time to wait for hook execution.
	Timeout time.Duration
	// SkipOnError stops hook execution on first error if false.
	SkipOnError bool
}

// DefaultManagerConfig returns the default hook manager configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Enabled:     false,
		Async:       true,
		Timeout:     5 * time.Second,
		SkipOnError: true,
	}
}

// Manager manages hook registration and execution.
type Manager struct {
	cfg    ManagerConfig
	hooks  []Hook
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewManager creates a new hook manager with the given configuration.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		cfg:    cfg,
		hooks:  make([]Hook, 0),
		logger: slog.With("component", "hook_manager"),
	}
}

// Register adds a hook to the manager.
// Hooks are executed in registration order.
func (m *Manager) Register(hooks ...Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hooks...)
	m.logger.Debug("Hook registered", "count", len(m.hooks))
}

// Unregister removes all hooks.
func (m *Manager) Unregister() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = make([]Hook, 0)
	m.logger.Debug("All hooks unregistered")
}

// Count returns the number of registered hooks.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks)
}

// Enabled returns true if hooks are enabled.
func (m *Manager) Enabled() bool {
	return m.cfg.Enabled && len(m.hooks) > 0
}

// executeHook executes a single hook with timeout and error handling.
func (m *Manager) executeHook(ctx context.Context, h Hook, execute func(Hook) error) error {
	if !m.cfg.Enabled {
		return nil
	}

	execCtx := ctx
	if m.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, m.cfg.Timeout)
		defer cancel()
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Hook panicked", "panic", r)
				done <- nil // Don't propagate panics
			}
		}()
		done <- execute(h)
	}()

	select {
	case err := <-done:
		return err
	case <-execCtx.Done():
		m.logger.Warn("Hook execution timeout", "timeout", m.cfg.Timeout)
		return nil // Don't fail on timeout
	}
}

// forEach executes the given function for each registered hook.
// If async is true, hooks are executed in parallel.
func (m *Manager) forEach(ctx context.Context, execute func(Hook) error, async bool) error {
	m.mu.RLock()
	hooks := append([]Hook(nil), m.hooks...)
	m.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	if async {
		// Execute hooks in parallel
		var wg sync.WaitGroup
		errChan := make(chan error, len(hooks))

		for _, h := range hooks {
			wg.Add(1)
			go func(hook Hook) {
				defer wg.Done()
				if err := m.executeHook(ctx, hook, execute); err != nil {
					errChan <- err
				}
			}(h)
		}

		wg.Wait()
		close(errChan)

		// Collect errors (non-blocking)
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
			if !m.cfg.SkipOnError {
				break
			}
		}

		if len(errors) > 0 {
			m.logger.Debug("Hook errors collected", "count", len(errors))
			if !m.cfg.SkipOnError {
				return errors[0]
			}
		}
	} else {
		// Execute hooks sequentially
		for _, h := range hooks {
			if err := m.executeHook(ctx, h, execute); err != nil {
				m.logger.Debug("Hook execution failed", "error", err)
				if !m.cfg.SkipOnError {
					return err
				}
			}
		}
	}

	return nil
}

// OnUserMessageBefore is called before processing user input.
// Returning an error from any hook will prevent message processing.
func (m *Manager) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnUserMessageBefore(ctx, hc, message)
	}, false) // Sequential for before hooks (allow blocking)
}

// OnUserMessageAfter is called after user input is processed.
func (m *Manager) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnUserMessageAfter(ctx, hc, message)
	}, m.cfg.Async)
}

// OnAssistantResponseBefore is called before sending assistant response.
// Returning an error from any hook will prevent response sending.
func (m *Manager) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnAssistantResponseBefore(ctx, hc, response)
	}, false) // Sequential for before hooks (allow blocking)
}

// OnAssistantResponseAfter is called after assistant response is sent.
func (m *Manager) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnAssistantResponseAfter(ctx, hc, response)
	}, m.cfg.Async)
}

// OnToolCallBefore is called before executing a tool.
// Returning an error from any hook will prevent tool execution.
func (m *Manager) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnToolCallBefore(ctx, hc, toolName, input)
	}, false) // Sequential for before hooks (allow blocking)
}

// OnToolCallAfter is called after tool execution completes.
func (m *Manager) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnToolCallAfter(ctx, hc, toolName, result)
	}, m.cfg.Async)
}

// OnStepStart is called when an agent step begins.
func (m *Manager) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnStepStart(ctx, hc, stepType)
	}, m.cfg.Async)
}

// OnStepEnd is called when an agent step completes.
func (m *Manager) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnStepEnd(ctx, hc, stepType, err)
	}, m.cfg.Async)
}

// OnError is called when an error occurs during agent execution.
func (m *Manager) OnError(ctx context.Context, hc HookContext, err error) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		return h.OnError(ctx, hc, err)
	}, m.cfg.Async)
}

// Publish publishes an event to all hooks (generic method).
// This is a lower-level method for custom event types.
func (m *Manager) Publish(ctx context.Context, event Event) error {
	if !m.Enabled() {
		return nil
	}

	return m.forEach(ctx, func(h Hook) error {
		// Hooks can optionally implement a generic OnEvent method
		// For now, this is a no-op for custom events
		return nil
	}, m.cfg.Async)
}
