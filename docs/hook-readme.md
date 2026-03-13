# Hook System

Non-intrusive hook system for monitoring and intercepting Crush agent lifecycle events.

## Quick Start

```go
package main

import (
    "context"
    "log/slog"
    "time"
    "github.com/fromsko/code/internal/hook"
)

func main() {
    // Create hook manager
    cfg := hook.ManagerConfig{
        Enabled:     true,
        Async:       true,
        Timeout:     5 * time.Second,
        SkipOnError: true,
    }
    manager := hook.NewManager(cfg)

    // Register built-in audit hook
    auditHook := hook.NewAuditHook(slog.Default())
    manager.Register(auditHook)

    // Register custom hook
    metricsHook := hook.NewMetricsHook(func(name string, value float64, labels ...string) {
        // Send to Prometheus
    })
    manager.Register(metricsHook)

    // Use manager in your agent
    // manager.OnUserMessageBefore(ctx, hc, &message)
    // manager.OnToolCallBefore(ctx, hc, toolName, input)
}
```

## Files

- `hook.go` - Core interfaces and types
- `manager.go` - Hook manager and execution logic
- `builtin.go` - Built-in hook implementations (Audit, Metrics, Timing)
- `config.go` - Configuration structures
- `hook_test.go` - Unit tests

## Documentation

See [docs/hook-system.md](../docs/hook-system.md) for complete documentation.

## Features

✅ Non-intrusive - Does not affect existing Fantasy callback system  
✅ Optional - Zero overhead when not configured  
✅ Async execution - Hooks run in background  
✅ Error isolation - Hook failures don't crash Crush  
✅ Pluggable - Dynamic load/unload support  
✅ Type-safe - Go interfaces with compile-time checking  

## Event Types

| Event | When | Blocking | Modifiable |
|-------|------|----------|------------|
| `UserMessageBefore` | Before processing user input | ✅ | ✅ |
| `UserMessageAfter` | After user input processed | ❌ | ❌ |
| `AssistantResponseBefore` | Before sending response | ✅ | ✅ |
| `AssistantResponseAfter` | After response sent | ❌ | ❌ |
| `ToolCallBefore` | Before tool execution | ✅ | ✅ |
| `ToolCallAfter` | After tool execution | ❌ | ❌ |
| `StepStart` | Step begins | ❌ | ❌ |
| `StepEnd` | Step completes | ❌ | ❌ |
| `Error` | Error occurs | ❌ | ❌ |

## Built-in Hooks

### AuditHook
Logs all events to slog.

```go
auditHook := hook.NewAuditHook(slog.Default())
```

### MetricsHook
Collects metrics (Prometheus-compatible).

```go
metricsHook := hook.NewMetricsHook(func(name string, value float64, labels ...string) {
    prometheus.Inc(name, value, labels...)
})
```

### TimingHook
Measures execution duration.

```go
timingHook := hook.NewTimingHook(func(name string, duration time.Duration, labels ...string) {
    histogram.Observe(name, duration.Seconds(), labels...)
})
```

## Configuration

```json
{
  "hooks": {
    "enabled": true,
    "async": true,
    "timeout": "5s",
    "skip_on_error": true,
    "plugins": [
      {
        "name": "audit",
        "path": "builtin:audit"
      },
      {
        "name": "metrics",
        "path": "./hooks/metrics.so",
        "config": {
          "prometheus_endpoint": "http://prometheus:9090"
        }
      }
    ]
  }
}
```

## License

Same as Crush project.
