package hook

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// AuditHook is a hook that logs all events for auditing purposes.
type AuditHook struct {
	logger *slog.Logger
}

// NewAuditHook creates a new audit hook with the given logger.
func NewAuditHook(logger *slog.Logger) *AuditHook {
	return &AuditHook{
		logger: logger.With("hook", "audit"),
	}
}

func (h *AuditHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	h.logger.Info("User message received",
		"session_id", hc.SessionID,
		"message_length", len(*message),
	)
	return nil
}

func (h *AuditHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	h.logger.Debug("User message processed",
		"session_id", hc.SessionID,
		"message_length", len(message),
	)
	return nil
}

func (h *AuditHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	h.logger.Info("Assistant response about to send",
		"session_id", hc.SessionID,
		"response_length", len(*response),
	)
	return nil
}

func (h *AuditHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	h.logger.Debug("Assistant response sent",
		"session_id", hc.SessionID,
		"response_length", len(response),
	)
	return nil
}

func (h *AuditHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	h.logger.Info("Tool call about to execute",
		"session_id", hc.SessionID,
		"tool_name", toolName,
		"tool_call_id", hc.ToolCallID,
	)
	return nil
}

func (h *AuditHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	h.logger.Debug("Tool call completed",
		"session_id", hc.SessionID,
		"tool_name", toolName,
		"tool_call_id", hc.ToolCallID,
	)
	return nil
}

func (h *AuditHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	h.logger.Debug("Step started",
		"session_id", hc.SessionID,
		"step_type", stepType,
	)
	return nil
}

func (h *AuditHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	if err != nil {
		h.logger.Warn("Step ended with error",
			"session_id", hc.SessionID,
			"step_type", stepType,
			"error", err,
		)
	} else {
		h.logger.Debug("Step completed",
			"session_id", hc.SessionID,
			"step_type", stepType,
		)
	}
	return nil
}

func (h *AuditHook) OnError(ctx context.Context, hc HookContext, err error) error {
	h.logger.Error("Error occurred",
		"session_id", hc.SessionID,
		"error", err,
	)
	return nil
}

// MetricsHook collects metrics for monitoring.
type MetricsHook struct {
	recordFunc func(name string, value float64, labels ...string)
}

// NewMetricsHook creates a new metrics hook.
// recordFunc should record metrics (e.g., Prometheus histogram, counter).
func NewMetricsHook(recordFunc func(name string, value float64, labels ...string)) *MetricsHook {
	return &MetricsHook{
		recordFunc: recordFunc,
	}
}

func (h *MetricsHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	if h.recordFunc != nil {
		h.recordFunc("hook_user_message_total", 1, "direction", "in")
		h.recordFunc("hook_user_message_bytes", float64(len(*message)))
	}
	return nil
}

func (h *MetricsHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	return nil
}

func (h *MetricsHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *MetricsHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	if h.recordFunc != nil {
		h.recordFunc("hook_assistant_response_total", 1, "direction", "out")
		h.recordFunc("hook_assistant_response_bytes", float64(len(response)))
	}
	return nil
}

func (h *MetricsHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	if h.recordFunc != nil {
		h.recordFunc("hook_tool_call_total", 1, "tool", toolName)
	}
	return nil
}

func (h *MetricsHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *MetricsHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	if h.recordFunc != nil {
		h.recordFunc("hook_step_start_total", 1, "step_type", stepType)
	}
	return nil
}

func (h *MetricsHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	if h.recordFunc != nil {
		labels := []string{"step_type", stepType}
		if err != nil {
			labels = append(labels, "status", "error")
		} else {
			labels = append(labels, "status", "success")
		}
		h.recordFunc("hook_step_end_total", 1, labels...)
	}
	return nil
}

func (h *MetricsHook) OnError(ctx context.Context, hc HookContext, err error) error {
	if h.recordFunc != nil {
		h.recordFunc("hook_error_total", 1)
	}
	return nil
}

// TimingHook measures execution time for operations.
type TimingHook struct {
	startTimes map[string]time.Time
	mu         sync.Map
	recordFunc func(name string, duration time.Duration, labels ...string)
}

// NewTimingHook creates a new timing hook.
func NewTimingHook(recordFunc func(name string, duration time.Duration, labels ...string)) *TimingHook {
	return &TimingHook{
		startTimes: make(map[string]time.Time),
		recordFunc: recordFunc,
	}
}

func (h *TimingHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	key := hc.SessionID + ":" + stepType
	h.mu.Store(key, time.Now())
	return nil
}

func (h *TimingHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	key := hc.SessionID + ":" + stepType
	if start, ok := h.mu.Load(key); ok {
		if startTime, ok := start.(time.Time); ok {
			duration := time.Since(startTime)
			if h.recordFunc != nil {
				h.recordFunc("hook_step_duration", duration, "step_type", stepType)
			}
			h.mu.Delete(key)
		}
	}
	return nil
}

func (h *TimingHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	key := hc.SessionID + ":tool:" + hc.ToolCallID
	h.mu.Store(key, time.Now())
	return nil
}

func (h *TimingHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	key := hc.SessionID + ":tool:" + hc.ToolCallID
	if start, ok := h.mu.Load(key); ok {
		if startTime, ok := start.(time.Time); ok {
			duration := time.Since(startTime)
			if h.recordFunc != nil {
				h.recordFunc("hook_tool_duration", duration, "tool", toolName)
			}
			h.mu.Delete(key)
		}
	}
	return nil
}

// No-op implementations for other methods
func (h *TimingHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	return nil
}

func (h *TimingHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	return nil
}

func (h *TimingHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *TimingHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *TimingHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}
