package hook

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNoopHook(t *testing.T) {
	t.Parallel()

	h := &NoopHook{}
	ctx := context.Background()
	hc := HookContext{}

	require.NoError(t, h.OnUserMessageBefore(ctx, hc, nil))
	require.NoError(t, h.OnUserMessageAfter(ctx, hc, ""))
	require.NoError(t, h.OnAssistantResponseBefore(ctx, hc, nil))
	require.NoError(t, h.OnAssistantResponseAfter(ctx, hc, ""))
	require.NoError(t, h.OnToolCallBefore(ctx, hc, "", nil))
	require.NoError(t, h.OnToolCallAfter(ctx, hc, "", nil))
	require.NoError(t, h.OnStepStart(ctx, hc, ""))
	require.NoError(t, h.OnStepEnd(ctx, hc, "", nil))
	require.NoError(t, h.OnError(ctx, hc, nil))
}

func TestHookManager_RegisterAndCount(t *testing.T) {
	t.Parallel()

	m := NewManager(ManagerConfig{Enabled: true})
	require.Equal(t, 0, m.Count())

	m.Register(&NoopHook{})
	require.Equal(t, 1, m.Count())

	m.Register(&NoopHook{}, &NoopHook{})
	require.Equal(t, 3, m.Count())
}

func TestHookManager_Enabled(t *testing.T) {
	t.Parallel()

	cfg := DefaultManagerConfig()
	cfg.Enabled = false
	m := NewManager(cfg)
	require.False(t, m.Enabled())

	cfg.Enabled = true
	m = NewManager(cfg)
	require.False(t, m.Enabled()) // No hooks registered

	m.Register(&NoopHook{})
	require.True(t, m.Enabled())
}

func TestHookManager_OnUserMessageBefore(t *testing.T) {
	t.Parallel()

	m := NewManager(ManagerConfig{Enabled: true})

	// Hook that modifies message
	modifyHook := &modifyMessageHook{modification: "[MODIFIED] "}
	m.Register(modifyHook)

	ctx := context.Background()
	hc := HookContext{SessionID: "test-session"}
	msg := "Hello"

	err := m.OnUserMessageBefore(ctx, hc, &msg)
	require.NoError(t, err)
	require.Equal(t, "[MODIFIED] Hello", msg)
}

func TestHookManager_OnUserMessageBefore_Blocking(t *testing.T) {
	t.Parallel()

	m := NewManager(ManagerConfig{Enabled: true})

	// Hook that blocks messages
	blockHook := &blockMessageHook{shouldBlock: true}
	m.Register(blockHook)

	ctx := context.Background()
	hc := HookContext{SessionID: "test-session"}
	msg := "Hello"

	err := m.OnUserMessageBefore(ctx, hc, &msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked")
}

func TestHookManager_OnToolCallBefore_Blocking(t *testing.T) {
	t.Parallel()

	m := NewManager(ManagerConfig{Enabled: true})

	// Hook that blocks dangerous commands
	blockHook := &blockToolHook{blockedTools: []string{"bash"}}
	m.Register(blockHook)

	ctx := context.Background()
	hc := HookContext{SessionID: "test-session", ToolCallID: "call-1"}

	err := m.OnToolCallBefore(ctx, hc, "bash", map[string]any{"command": "rm -rf /"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked")

	// Non-blocked tool should pass
	err = m.OnToolCallBefore(ctx, hc, "view", map[string]any{"path": "/test"})
	require.NoError(t, err)
}

func TestHookManager_AsyncExecution(t *testing.T) {
	t.Parallel()

	cfg := DefaultManagerConfig()
	cfg.Async = true
	m := NewManager(cfg)

	var mu sync.Mutex
	executionOrder := []string{}

	slowHook := &slowHook{delay: 100 * time.Millisecond, name: "slow"}
	fastHook := &fastHook{name: "fast", order: &executionOrder, mu: &mu}

	m.Register(slowHook, fastHook)

	ctx := context.Background()
	hc := HookContext{SessionID: "test-session"}

	start := time.Now()
	err := m.OnUserMessageAfter(ctx, hc, "test")
	require.NoError(t, err)
	elapsed := time.Since(start)

	// Should complete in ~100ms (parallel), not ~200ms (sequential)
	require.Less(t, elapsed, 150*time.Millisecond)
}

func TestHookManager_Timeout(t *testing.T) {
	t.Parallel()

	cfg := ManagerConfig{
		Enabled:     true,
		Async:       false,
		Timeout:     50 * time.Millisecond,
		SkipOnError: true,
	}
	m := NewManager(cfg)

	slowHook := &slowHook{delay: 200 * time.Millisecond, name: "slow"}
	m.Register(slowHook)

	ctx := context.Background()
	hc := HookContext{SessionID: "test-session"}

	// Should timeout but not fail
	err := m.OnUserMessageAfter(ctx, hc, "test")
	require.NoError(t, err)
}

func TestHookManager_ErrorHandling(t *testing.T) {
	t.Parallel()

	cfg := DefaultManagerConfig()
	cfg.SkipOnError = true
	m := NewManager(cfg)

	errorHook := &errorHook{shouldError: true}
	noopHook := &NoopHook{}

	m.Register(errorHook, noopHook)

	ctx := context.Background()
	hc := HookContext{SessionID: "test-session"}

	// First hook errors, but SkipOnError=true so it continues
	err := m.OnUserMessageAfter(ctx, hc, "test")
	require.NoError(t, err) // Error is swallowed due to SkipOnError
}

func TestHookManager_Unregister(t *testing.T) {
	t.Parallel()

	m := NewManager(ManagerConfig{Enabled: true})
	m.Register(&NoopHook{}, &NoopHook{})
	require.Equal(t, 2, m.Count())

	m.Unregister()
	require.Equal(t, 0, m.Count())
}

// Test hooks implementation

type modifyMessageHook struct {
	modification string
}

func (h *modifyMessageHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	*message = h.modification + *message
	return nil
}

func (h *modifyMessageHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	return nil
}

func (h *modifyMessageHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *modifyMessageHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *modifyMessageHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	return nil
}

func (h *modifyMessageHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *modifyMessageHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *modifyMessageHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *modifyMessageHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}

type blockMessageHook struct {
	shouldBlock bool
}

func (h *blockMessageHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	if h.shouldBlock {
		return errors.New("message blocked")
	}
	return nil
}

func (h *blockMessageHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	return nil
}

func (h *blockMessageHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *blockMessageHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *blockMessageHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	return nil
}

func (h *blockMessageHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *blockMessageHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *blockMessageHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *blockMessageHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}

type blockToolHook struct {
	blockedTools []string
}

func (h *blockToolHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	return nil
}

func (h *blockToolHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	return nil
}

func (h *blockToolHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *blockToolHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *blockToolHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	for _, blocked := range h.blockedTools {
		if toolName == blocked {
			return errors.New("tool blocked")
		}
	}
	return nil
}

func (h *blockToolHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *blockToolHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *blockToolHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *blockToolHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}

type slowHook struct {
	delay time.Duration
	name  string
}

func (h *slowHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	time.Sleep(h.delay)
	return nil
}

func (h *slowHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	return nil
}

func (h *slowHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *slowHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *slowHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	return nil
}

func (h *slowHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *slowHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *slowHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *slowHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}

type fastHook struct {
	name  string
	order *[]string
	mu    *sync.Mutex
}

func (h *fastHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.order = append(*h.order, h.name)
	return nil
}

func (h *fastHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	return nil
}

func (h *fastHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *fastHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *fastHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	return nil
}

func (h *fastHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *fastHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *fastHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *fastHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}

type errorHook struct {
	shouldError bool
}

func (h *errorHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	if h.shouldError {
		return errors.New("test error")
	}
	return nil
}

func (h *errorHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	return nil
}

func (h *errorHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *errorHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *errorHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	return nil
}

func (h *errorHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *errorHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *errorHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *errorHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}
