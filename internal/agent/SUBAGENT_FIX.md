# SubAgent 错误处理修复说明

## 修复概述

本次修复针对 subAgent 调用过程中的错误处理、超时控制、重试机制和日志记录进行了全面改进。

## 修复内容

### 1. 新增错误类型 (`errors.go`)

**文件**: `internal/agent/errors.go`

新增 `SubAgentError` 类型，用于包装 subAgent 执行过程中的错误，提供以下功能：

- **错误上下文**: 包含操作类型 (`Op`) 和会话 ID (`Session`)
- **错误链**: 实现 `Unwrap()` 方法，支持 `errors.Is` 和 `errors.As`
- **辅助函数**:
  - `NewSubAgentError(op, sessionID, err)`: 创建新的 SubAgentError
  - `IsSubAgentError(err)`: 检查错误是否为 SubAgentError

**使用示例**:
```go
if err != nil {
    return fantasy.ToolResponse{}, NewSubAgentError("create_session", session.ID, err)
}
```

### 2. 超时控制 (`coordinator.go`)

**文件**: `internal/agent/coordinator.go`

新增常量：
```go
const (
    subAgentTimeout = 10 * time.Minute  // subAgent 执行超时
)
```

在 `runSubAgent` 函数中使用 `context.WithTimeout` 创建超时上下文：
```go
subAgentCtx, cancel := context.WithTimeout(ctx, subAgentTimeout)
defer cancel()
```

**特性**:
- 默认超时时间：10 分钟
- 超时后自动取消 subAgent 执行
- 超时错误会被正确捕获并格式化为用户友好的消息

### 3. 重试机制 (`coordinator.go`)

**文件**: `internal/agent/coordinator.go`

新增常量：
```go
const (
    subAgentMaxRetries = 2              // 最大重试次数
    subAgentRetryDelay = 1 * time.Second // 初始重试延迟
)
```

**重试逻辑**:
- 仅在检测到可重试错误时进行重试
- 使用指数退避策略：`delay = baseDelay * 2^(attempt-1)`
- 最大重试次数：2 次（共 3 次尝试）

**可重试错误类型** (`isRetryableError` 函数):
- 临时网络错误 (`net.Error.Temporary()`)
- 网络超时错误 (`net.Error.Timeout()`)
- HTTP 5xx 服务器错误
- HTTP 429 速率限制错误
- 特定错误消息模式：
  - "connection reset"
  - "broken pipe"
  - "network is unreachable"
  - "timeout"
  - "temporary failure"
  - "i/o timeout"

**不可重试错误**:
- 上下文取消 (`context.Canceled`)
- 上下文超时 (`context.DeadlineExceeded`)
- HTTP 4xx 客户端错误（除 429 外）

### 4. 错误信息改进 (`coordinator.go`)

**文件**: `internal/agent/coordinator.go`

新增 `formatSubAgentError` 函数，将内部错误转换为用户友好的消息：

| 错误类型 | 用户消息 |
|---------|---------|
| SubAgentError | "Sub-agent failed to {op}: {err}" |
| context.DeadlineExceeded | "Sub-agent timed out after 10m0s" |
| context.Canceled | "Sub-agent was canceled by user" |
| 其他错误 | "Sub-agent error: {err}" |

**改进点**:
- 原始错误被丢弃的问题已修复
- 错误消息清晰说明失败原因
- 保留足够的调试信息用于日志记录

### 5. 日志记录改进 (`coordinator.go`)

**文件**: `internal/agent/coordinator.go`

新增日志记录点：

```go
// 会话创建
slog.Debug("Sub-agent session created", 
    "session_id", session.ID, 
    "parent_session", params.SessionID, 
    "title", params.SessionTitle)

// 重试时
slog.Debug("Retrying sub-agent execution", 
    "session_id", session.ID, 
    "attempt", attempt, 
    "delay", delay)

// 可重试错误
slog.Debug("Sub-agent execution failed with retryable error", 
    "session_id", session.ID, 
    "attempt", attempt+1, 
    "error", runErr)

// 最终失败
slog.Error("Sub-agent execution failed", 
    "session_id", session.ID, 
    "error", runErr, 
    "attempts", subAgentMaxRetries+1)

// 成本更新失败
slog.Error("Failed to update parent session cost", 
    "child_session", session.ID, 
    "parent_session", params.SessionID, 
    "error", err)

// 成功完成
slog.Debug("Sub-agent completed successfully", 
    "session_id", session.ID, 
    "parent_session", params.SessionID)
```

### 6. 资源清理

**改进点**:
- 使用 `defer cancel()` 确保超时上下文总是被取消
- 子会话创建后立即记录日志，便于追踪
- 错误情况下也会尝试更新父会话成本（如果可能）
- 所有错误路径都有适当的日志记录

### 7. 并发安全

**现有机制**:
- 使用 `context` 进行取消传播
- session 和 message 服务已经是并发安全的
- 活跃请求跟踪通过 `csync.Map` 实现

**改进**:
- 超时上下文确保 subAgent 不会无限期运行
- 重试逻辑中检查上下文取消状态

## 测试用例

**文件**: `internal/agent/subagent_test.go`

新增测试：

1. **TestSubAgentError**: 测试 SubAgentError 类型
   - 错误消息格式（带/不带会话 ID）
   - Unwrap 方法
   - IsSubAgentError 函数

2. **TestIsRetryableError**: 测试可重试错误检测
   - nil 错误
   - 上下文错误（canceled, deadline exceeded）
   - 网络错误（temporary, timeout）
   - HTTP 错误（5xx, 429, 4xx）
   - 错误消息模式匹配

3. **TestFormatSubAgentError**: 测试错误格式化
   - SubAgentError
   - 上下文错误
   - 通用错误

4. **TestSubAgentTimeoutConstant**: 验证超时常量合理性

5. **TestSubAgentRetryConstants**: 验证重试常量合理性

## 验证命令

```bash
# 构建项目
go build ./...

# 运行测试（包括竞态检测）
go test -race ./internal/agent/...

# 运行新增的测试
go test -v ./internal/agent/subagent_test.go
```

## 影响范围

### 修改的文件
- `internal/agent/errors.go` - 新增错误类型
- `internal/agent/coordinator.go` - 修复 runSubAgent 函数
- `internal/agent/subagent_test.go` - 新增测试

### 受影响的功能
- `agent` 工具（subAgent 调用）
- `agentic_fetch` 工具（使用 subAgent）

### 向后兼容性
- ✅ API 无破坏性变更
- ✅ 现有功能保持不变
- ✅ 错误处理更加健壮
- ✅ 用户看到更清晰的错误消息

## 配置建议

如果需要调整 subAgent 行为，可以修改以下常量：

```go
const (
    subAgentTimeout = 10 * time.Minute  // 根据任务复杂度调整
    subAgentMaxRetries = 2              // 根据网络稳定性调整
    subAgentRetryDelay = 1 * time.Second // 根据 API 响应时间调整
)
```

## 监控建议

建议在生产环境中监控以下日志：

1. **错误日志** (`slog.Error`):
   - "Sub-agent execution failed" - 频繁出现可能需要调整重试策略
   - "Failed to update parent session cost" - 可能表示数据一致性问题

2. **调试日志** (`slog.Debug`):
   - 重试频率高可能表示网络问题
   - 超时频繁可能需要增加 timeout 或优化 subAgent 任务

## 故障排查

### SubAgent 频繁超时
1. 检查 `subAgentTimeout` 配置
2. 查看日志中的错误类型
3. 考虑优化 subAgent 的 prompt 或工具集

### SubAgent 频繁重试
1. 检查网络连接稳定性
2. 查看 provider 状态（是否有 API 问题）
3. 考虑调整 `subAgentMaxRetries` 或 `subAgentRetryDelay`

### 错误信息不清晰
1. 检查日志中的详细错误
2. 确认 `formatSubAgentError` 处理了所有错误类型
3. 必要时添加新的错误模式匹配

## 总结

本次修复全面改进了 subAgent 的错误处理机制：

✅ 错误被正确捕获和报告  
✅ 添加了 10 分钟超时控制  
✅ 实现了智能重试机制（网络错误自动重试）  
✅ 改进了错误信息，用户能看到详细错误  
✅ 确保资源正确清理（defer cancel）  
✅ 添加了充分的日志记录  
✅ 并发安全  
✅ 有回退机制（错误时返回友好消息）
