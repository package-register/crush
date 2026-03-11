# SubAgent 错误处理修复文档

## 概述

本文档描述了对 subAgent 调用错误处理、超时控制、重试机制和日志记录的修复。

## 修复内容

### 1. 错误处理改进

#### `internal/agent/errors.go`
- ✅ 已存在 `SubAgentError` 类型，包含操作、会话 ID 和底层错误
- ✅ 提供 `NewSubAgentError()` 构造函数
- ✅ 提供 `IsSubAgentError()` 检查函数
- ✅ 实现 `Unwrap()` 方法支持错误链

#### `internal/agent/coordinator.go` - `runSubAgent` 函数
- ✅ 错误被正确包装为 `SubAgentError`
- ✅ 错误信息包含会话 ID 和操作类型
- ✅ 用户友好的错误消息通过 `formatSubAgentError()` 生成

### 2. 超时控制

```go
// 常量定义
const (
    subAgentTimeout = 10 * time.Minute  // subAgent 执行超时
    subAgentMaxRetries = 2              // 最大重试次数
    subAgentRetryDelay = 1 * time.Second // 初始重试延迟
)

// 超时上下文
subAgentCtx, cancel := context.WithTimeout(ctx, subAgentTimeout)
defer cancel()
```

### 3. 重试机制

```go
for attempt = 0; attempt <= subAgentMaxRetries; attempt++ {
    if attempt > 0 {
        // 指数退避延迟
        delay := subAgentRetryDelay * time.Duration(1<<uint(attempt-1))
        select {
        case <-time.After(delay):
        case <-subAgentCtx.Done():
            return NewSubAgentError("execute", session.ID, subAgentCtx.Err())
        }
    }

    result, runErr = params.Agent.Run(subAgentCtx, ...)
    
    if runErr == nil {
        break // 成功
    }
    
    if !c.isRetryableError(runErr) {
        break // 非可重试错误
    }
}
```

#### 可重试错误类型 (`isRetryableError`)
- ✅ 网络超时错误 (`net.Error.Timeout()`)
- ✅ HTTP 5xx 服务器错误
- ✅ HTTP 429 速率限制错误
- ✅ 特定错误消息模式：
  - "connection reset"
  - "broken pipe"
  - "network is unreachable"
  - "timeout"
  - "temporary failure"
  - "i/o timeout"

#### 不可重试错误
- ❌ 上下文取消 (`context.Canceled`)
- ❌ 上下文超时 (`context.DeadlineExceeded`)
- ❌ HTTP 4xx 客户端错误（除 429 外）

### 4. 日志记录改进

使用 `slog` 包添加全面的日志记录：

```go
// 会话创建
slog.Info("Sub-agent session created", 
    "session_id", session.ID, 
    "parent_session", params.SessionID, 
    "title", params.SessionTitle,
    "prompt_length", len(params.Prompt))

// 开始执行
slog.Info("Starting sub-agent execution", 
    "session_id", session.ID, 
    "timeout", subAgentTimeout, 
    "max_retries", subAgentMaxRetries)

// 重试
slog.Warn("Retrying sub-agent execution", 
    "session_id", session.ID, 
    "attempt", attempt, 
    "max_retries", subAgentMaxRetries, 
    "delay", delay, 
    "error", runErr)

// 成功
slog.Info("Sub-agent completed successfully", 
    "session_id", session.ID, 
    "parent_session", params.SessionID,
    "prompt_length", len(params.Prompt))

// 失败
slog.Error("Sub-agent execution failed after all retries", 
    "session_id", session.ID, 
    "parent_session", params.SessionID, 
    "error", runErr, 
    "total_attempts", attempt+1)
```

### 5. 资源清理

- ✅ 使用 `defer cancel()` 确保超时上下文被正确清理
- ✅ 错误情况下会话仍然正确创建和清理
- ✅ 成本累积失败不影响主流程

### 6. Sourcegraph 工具错误处理改进

#### `internal/agent/tools/sourcegraph.go`
- ✅ 添加请求创建失败的日志记录
- ✅ 添加请求执行失败的日志记录
- ✅ 添加响应读取失败的日志记录
- ✅ 添加 JSON 解析失败的日志记录
- ✅ 添加结果格式化失败的日志记录
- ✅ 使用 `defer resp.Body.Close()` 确保资源清理

## 错误流程

```
用户调用 subAgent
    ↓
创建子会话 (失败 → NewSubAgentError("create_session"))
    ↓
获取提供者配置 (失败 → NewSubAgentError("get_provider"))
    ↓
创建超时上下文
    ↓
重试循环 (最多 subAgentMaxRetries+1 次)
    ↓
    执行 Agent.Run()
    ↓
    成功？→ 跳出循环
    ↓
    失败 → 检查是否可重试
        ↓
        可重试？→ 等待延迟 → 下次重试
        ↓
        不可重试？→ 跳出循环
    ↓
最终检查
    ↓
    成功 → 更新父会话成本 → 返回结果
    ↓
    失败 → formatSubAgentError() → 返回错误响应
```

## 用户可见的错误消息

| 错误类型 | 用户消息 |
|---------|---------|
| SubAgentError | `Sub-agent failed to {op}: {err}` |
| 上下文超时 | `Sub-agent timed out after 10m0s` |
| 上下文取消 | `Sub-agent was canceled by user` |
| 其他错误 | `Sub-agent error: {err}` |

## 测试用例

现有测试覆盖：
- ✅ `TestSubAgentError` - SubAgentError 类型测试
- ✅ `TestIsRetryableError` - 可重试错误检测测试
- ✅ `TestFormatSubAgentError` - 错误格式化测试
- ✅ `TestRunSubAgent` - runSubAgent 功能测试
- ✅ `TestUpdateParentSessionCost` - 成本累积测试
- ✅ `TestSubAgentTimeoutConstant` - 超时常量测试
- ✅ `TestSubAgentRetryConstants` - 重试常量测试

## 验证命令

```bash
go build ./...
go test -race ./internal/agent/...
```

## 相关文件

- `internal/agent/coordinator.go` - 主要修复位置
- `internal/agent/errors.go` - 错误类型定义
- `internal/agent/tools/sourcegraph.go` - 工具错误处理改进
- `internal/agent/subagent_test.go` - 单元测试
- `internal/agent/coordinator_test.go` - 集成测试

## 注意事项

1. **不破坏现有功能**：所有修改都是向后兼容的
2. **并发安全**：使用 context 控制超时和取消
3. **回退机制**：错误情况下返回友好的错误消息
4. **日志级别**：
   - `Info` - 正常流程（开始、完成）
   - `Debug` - 详细信息（每次尝试）
   - `Warn` - 重试情况
   - `Error` - 失败情况
