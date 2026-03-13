# Hook 系统文档

## 概述

Hook 系统提供了一个**非侵入式**的机制，用于监控和拦截 Crush agent 的生命周期事件。Hook 是可选的，可用于审计、指标收集、合规检查等跨切面关注点。

## 核心特性

- ✅ **非侵入式** - 不影响现有 Fantasy 回调系统
- ✅ **可选启用** - 不配置时零性能损耗
- ✅ **异步执行** - Hook 在后台运行，不阻塞主流程
- ✅ **错误隔离** - Hook 失败不影响 Crush 运行
- ✅ **可插拔** - 支持动态加载/卸载 Hook
- ✅ **类型安全** - Go 接口定义，编译时检查

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                    Crush Core                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   Fantasy   │  │   Session   │  │    Tools    │     │
│  │  Callbacks  │  │   Manager   │  │   System    │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│                          │                              │
│                 ┌────────▼────────┐                     │
│                 │  HookManager    │ ◄── 可选组件        │
│                 └────────┬────────┘                     │
└──────────────────────────│──────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│  Audit Hook   │  │ Metrics Hook  │  │ Billing Hook  │
│  (日志审计)    │  │  (指标收集)    │  │  (计费统计)    │
└───────────────┘  └───────────────┘  └───────────────┘
```

## Hook 生命周期事件

### 用户消息事件

| 事件 | 调用时机 | 可拦截 | 可修改 |
|------|---------|--------|--------|
| `UserMessageBefore` | 收到用户消息后，处理前 | ✅ | ✅ |
| `UserMessageAfter` | 用户消息处理完成后 | ❌ | ❌ |

### 助手响应事件

| 事件 | 调用时机 | 可拦截 | 可修改 |
|------|---------|--------|--------|
| `AssistantResponseBefore` | 发送响应前 | ✅ | ✅ |
| `AssistantResponseAfter` | 响应发送后 | ❌ | ❌ |

### 工具调用事件

| 事件 | 调用时机 | 可拦截 | 可修改 |
|------|---------|--------|--------|
| `ToolCallBefore` | 工具执行前 | ✅ | ✅ |
| `ToolCallAfter` | 工具执行后 | ❌ | ❌ |

### Step 生命周期事件

| 事件 | 调用时机 | 可拦截 |
|------|---------|--------|
| `StepStart` | Step 开始时 | ❌ |
| `StepEnd` | Step 结束时 | ❌ |

### 错误事件

| 事件 | 调用时机 |
|------|---------|
| `Error` | 发生错误时 |

## 使用方式

### 1. 基础使用

```go
package main

import (
    "context"
    "log/slog"
    "github.com/fromsko/code/internal/hook"
)

func main() {
    // 创建 Hook Manager
    cfg := hook.ManagerConfig{
        Enabled:     true,
        Async:       true,
        Timeout:     5 * time.Second,
        SkipOnError: true,
    }
    manager := hook.NewManager(cfg)

    // 注册内置 Audit Hook
    auditHook := hook.NewAuditHook(slog.Default())
    manager.Register(auditHook)

    // Hook 现在会在事件发生时自动触发
}
```

### 2. 自定义 Hook

```go
// 敏感词过滤 Hook
type ComplianceHook struct {
    keywords []string
}

func (h *ComplianceHook) OnUserMessageBefore(ctx context.Context, hc hook.HookContext, message *string) error {
    for _, kw := range h.keywords {
        if strings.Contains(*message, kw) {
            return fmt.Errorf("message contains prohibited keyword: %s", kw)
        }
    }
    return nil
}

// 其他方法可以嵌入 NoopHook 简化实现
type ComplianceHook struct {
    hook.NoopHook  // 嵌入 NoopHook，只需实现需要的方法
    keywords []string
}
```

### 3. 指标收集 Hook

```go
type MetricsHook struct {
    prometheus *prometheus.Client
}

func (h *MetricsHook) OnToolCallBefore(ctx context.Context, hc hook.HookContext, toolName string, input any) error {
    h.prometheus.Inc("tool_calls_total", "tool", toolName)
    return nil
}

func (h *MetricsHook) OnStepEnd(ctx context.Context, hc hook.HookContext, stepType string, err error) error {
    duration := ctx.Value("step_duration").(time.Duration)
    h.prometheus.Observe("step_duration_seconds", duration.Seconds(), "type", stepType)
    if err != nil {
        h.prometheus.Inc("step_errors_total", "type", stepType)
    }
    return nil
}
```

### 4. 远程 Hook (gRPC)

```go
type RemoteHook struct {
    client hookpb.HookServiceClient
}

func (h *RemoteHook) OnUserMessageBefore(ctx context.Context, hc hook.HookContext, message *string) error {
    resp, err := h.client.ValidateMessage(ctx, &hookpb.ValidateMessageRequest{
        SessionId: hc.SessionID,
        Message:   *message,
    })
    if err != nil {
        return err
    }
    if !resp.Allowed {
        return fmt.Errorf("message blocked: %s", resp.Reason)
    }
    return nil
}
```

## 配置示例

### crush.json 配置

```json
{
  "$schema": "https://fromsko.code/crush.json",
  "hooks": {
    "enabled": true,
    "async": true,
    "timeout": "5s",
    "skip_on_error": true,
    "plugins": [
      {
        "name": "audit",
        "path": "builtin:audit",
        "config": {
          "log_level": "info"
        }
      },
      {
        "name": "metrics",
        "path": "./hooks/metrics.so",
        "config": {
          "prometheus_endpoint": "http://prometheus:9090"
        }
      }
    ],
    "remote": {
      "enabled": true,
      "endpoint": "grpc://hook-server.internal:50051",
      "events": ["tool_call_before", "user_message_before"],
      "timeout": "3s"
    }
  }
}
```

## 内置 Hook

### AuditHook

记录所有事件到日志系统。

```go
auditHook := hook.NewAuditHook(slog.Default())
```

**日志输出示例**：
```
INFO Hook: audit User message received session_id=abc123 message_length=256
INFO Hook: audit Tool call about to execute session_id=abc123 tool_name=bash
INFO Hook: audit Assistant response about to send session_id=abc123 response_length=1024
```

### MetricsHook

收集指标数据。

```go
metricsHook := hook.NewMetricsHook(func(name string, value float64, labels ...string) {
    prometheus.Inc(name, value, labels...)
})
```

**收集的指标**：
- `hook_user_message_total` - 用户消息计数
- `hook_user_message_bytes` - 用户消息大小
- `hook_assistant_response_total` - 助手响应计数
- `hook_assistant_response_bytes` - 响应大小
- `hook_tool_call_total` - 工具调用计数
- `hook_step_start_total` - Step 开始计数
- `hook_step_end_total` - Step 结束计数

### TimingHook

测量执行时间。

```go
timingHook := hook.NewTimingHook(func(name string, duration time.Duration, labels ...string) {
    histogram.Observe(name, duration.Seconds(), labels...)
})
```

**测量的时间**：
- `hook_step_duration` - Step 执行时间
- `hook_tool_duration` - 工具执行时间

## 最佳实践

### 1. Hook 性能

```go
// ✅ 好的做法：异步执行耗时操作
func (h *MyHook) OnToolCallAfter(ctx context.Context, hc hook.HookContext, toolName string, result any) error {
    go h.sendToExternalSystem(result)  // 异步发送
    return nil
}

// ❌ 不好的做法：阻塞主流程
func (h *MyHook) OnToolCallAfter(ctx context.Context, hc hook.HookContext, toolName string, result any) error {
    return h.sendToExternalSystem(result)  // 同步阻塞
}
```

### 2. 错误处理

```go
// ✅ 好的做法：返回 error 阻止操作
func (h *SecurityHook) OnToolCallBefore(ctx context.Context, hc hook.HookContext, toolName string, input any) error {
    if toolName == "bash" && isDangerousCommand(input) {
        return fmt.Errorf("dangerous command blocked")
    }
    return nil
}

// ✅ 好的做法：记录错误但不阻止
func (h *MetricsHook) OnError(ctx context.Context, hc HookContext, err error) error {
    h.prometheus.Inc("errors_total")
    return nil  // 不阻止错误传播
}
```

### 3. 上下文使用

```go
// ✅ 使用 HookContext 传递信息
func (h *MyHook) OnStepStart(ctx context.Context, hc hook.HookContext, stepType string) error {
    ctx = context.WithValue(ctx, "step_start_time", time.Now())
    return nil
}
```

## 调试

### 启用调试日志

```json
{
  "options": {
    "debug": true
  },
  "hooks": {
    "enabled": true
  }
}
```

### 查看 Hook 执行情况

```bash
# 查看 Hook 相关日志
crush logs | grep hook

# 查看特定 Hook 的执行
crush logs | grep "hook_manager"
```

## 故障排查

### Hook 未触发

1. 检查 `hooks.enabled` 是否为 `true`
2. 检查是否注册了 Hook (`manager.Count() > 0`)
3. 检查事件类型是否在 Hook 中实现

### Hook 阻塞主流程

1. 设置 `hooks.async = true`
2. 检查 Hook 是否有耗时操作
3. 降低 `hooks.timeout`

### Hook 错误被忽略

- 默认 `skip_on_error = true`（错误被记录但不影响主流程）
- 设置 `skip_on_error = false` 可使 Hook 错误中断操作

## 扩展开发

### 创建 Hook 插件

```go
package main  // 必须是 main 包

import (
    "github.com/fromsko/code/internal/hook"
)

// 导出 Hook 实例
var Hook = &MyCustomHook{}

type MyCustomHook struct {
    hook.NoopHook
}

func (h *MyCustomHook) OnUserMessageBefore(ctx context.Context, hc hook.HookContext, message *string) error {
    // 实现逻辑
    return nil
}
```

### 编译插件

```bash
go build -buildmode=plugin -o myhook.so ./hooks/myhook
```

### 加载插件

```json
{
  "hooks": {
    "plugins": [
      {
        "path": "./myhook.so"
      }
    ]
  }
}
```

## API 参考

### Hook 接口

```go
type Hook interface {
    OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error
    OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error
    OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error
    OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error
    OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error
    OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error
    OnStepStart(ctx context.Context, hc HookContext, stepType string) error
    OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error
    OnError(ctx context.Context, hc HookContext, err error) error
}
```

### Manager 方法

```go
func NewManager(cfg ManagerConfig) *Manager
func (m *Manager) Register(hooks ...Hook)
func (m *Manager) Count() int
func (m *Manager) Enabled() bool
```

## 相关文档

- [SubAgent 机制](subagent-mechanism.md)
- [AG-UI 协议](../internal/agui-server/README.md)
- [PubSub 系统](../internal/pubsub/README.md)
