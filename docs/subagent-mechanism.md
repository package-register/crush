# SubAgent 机制文档

## 概述

SubAgent（子代理）机制允许主 Agent 在执行任务时创建并调用独立的子代理会话，实现任务分解和并行处理。

## 核心实现

### 文件结构

```
internal/agent/
├── agent_tool.go          # agent 工具实现（调用 SubAgent）
├── coordinator.go         # runSubAgent() 核心实现
├── errors.go              # SubAgentError 错误类型
├── subagent_test.go       # 单元测试
├── agentic_fetch_tool.go  # 使用 SubAgent 的网页抓取工具
└── templates/
    └── agent_tool.md      # agent 工具描述
```

### 调用流程

```
用户请求
    ↓
主 Agent 运行
    ↓
调用 agent 工具 (agent_tool.go)
    ↓
runSubAgent() (coordinator.go)
    ↓
创建子会话 → 执行子代理 → 返回结果
    ↓
更新父会话成本
    ↓
返回 ToolResponse
```

## 关键特性

### 1. 超时控制

```go
const subAgentTimeout = 10 * time.Minute
```

- 默认超时：**10 分钟**
- 使用 `context.WithTimeout` 实现
- 超时后自动取消 SubAgent 执行
- 超时错误会被格式化为用户友好的消息

### 2. 重试机制

```go
const (
    subAgentMaxRetries = 2
    subAgentRetryDelay = 1 * time.Second
)
```

| 参数 | 值 | 说明 |
|------|-----|------|
| `subAgentMaxRetries` | 2 | 最大重试次数（共 3 次尝试） |
| `subAgentRetryDelay` | 1s | 初始重试延迟 |

**重试策略**：
- 指数退避：`delay = baseDelay * 2^(attempt-1)`
- 仅对可重试错误进行重试

**可重试错误类型**：
- 临时网络错误 (`net.Error.Temporary()`)
- 网络超时错误 (`net.Error.Timeout()`)
- HTTP 5xx 服务器错误
- HTTP 429 速率限制错误
- 错误消息包含：`"connection reset"`, `"broken pipe"`, `"network is unreachable"`, `"timeout"`, `"temporary failure"`, `"i/o timeout"`

**不可重试错误**：
- 上下文取消 (`context.Canceled`)
- 上下文超时 (`context.DeadlineExceeded`)
- HTTP 4xx 客户端错误（除 429 外）

### 3. 错误处理

**SubAgentError 类型** (`errors.go`)：

```go
type SubAgentError struct {
    Op      string // 操作类型（如 "create_session", "execute"）
    Session string // 会话 ID
    Err     error  // 底层错误
}
```

**错误格式化** (`formatSubAgentError` 函数)：

| 错误类型 | 用户消息 |
|---------|---------|
| `SubAgentError` | "Sub-agent failed to {op}: {err}" |
| `context.DeadlineExceeded` | "Sub-agent timed out after 10m0s" |
| `context.Canceled` | "Sub-agent was canceled by user" |
| 其他错误 | "Sub-agent error: {err}" |

### 4. 日志记录

关键日志点：

```go
// 会话创建
slog.Debug("Sub-agent session created", 
    "session_id", session.ID, 
    "parent_session", params.SessionID, 
    "title", params.SessionTitle)

// 重试
slog.Debug("Retrying sub-agent execution", 
    "session_id", session.ID, 
    "attempt", attempt, 
    "delay", delay)

// 最终失败
slog.Error("Sub-agent execution failed", 
    "session_id", session.ID, 
    "error", runErr, 
    "attempts", subAgentMaxRetries+1)

// 成功完成
slog.Debug("Sub-agent completed successfully", 
    "session_id", session.ID, 
    "parent_session", params.SessionID)
```

## 使用场景

### 1. agent 工具

**文件**: `internal/agent/agent_tool.go`

```go
func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
    // 创建 task agent
    agent, err := c.buildAgent(ctx, prompt, agentCfg, true)
    
    // 返回并行 agent 工具
    return fantasy.NewParallelAgentTool(
        AgentToolName,
        string(agentToolDescription),
        func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
            return c.runSubAgent(ctx, subAgentParams{
                Agent:          agent,
                SessionID:      sessionID,
                AgentMessageID: agentMessageID,
                ToolCallID:     call.ID,
                Prompt:         params.Prompt,
                SessionTitle:   "New Agent Session",
            })
        }), nil
}
```

**使用方式**：
```
用户：帮我分析这个项目的测试覆盖率
Agent：调用 agent 工具 → 创建 SubAgent 专门处理测试分析
```

### 2. agentic_fetch 工具

**文件**: `internal/agent/agentic_fetch_tool.go`

```go
// 使用 SubAgent 分析网页内容
result, err := c.runSubAgent(ctx, subAgentParams{
    Agent:          agent,
    SessionID:      sessionID,
    AgentMessageID: messageID,
    ToolCallID:     call.ID,
    Prompt:         fetchPrompt,
    SessionTitle:   "Web Content Analysis",
    IsSubAgent:     true,
})
```

**使用方式**：
```
用户：分析一下 https://example.com 的内容
Agent：调用 agentic_fetch → 创建 SubAgent 分析网页内容
```

## 配置参数

### 常量配置

在 `internal/agent/coordinator.go` 中修改：

```go
const (
    subAgentTimeout = 10 * time.Minute  // 根据任务复杂度调整
    subAgentMaxRetries = 2              // 根据网络稳定性调整
    subAgentRetryDelay = 1 * time.Second // 根据 API 响应时间调整
)
```

### 建议配置

| 场景 | Timeout | MaxRetries | RetryDelay |
|------|---------|------------|------------|
| 快速查询任务 | 5m | 1 | 500ms |
| 代码分析任务 | 10m | 2 | 1s |
| 复杂重构任务 | 20m | 3 | 2s |
| 网络不稳定环境 | 15m | 5 | 3s |

## 监控与故障排查

### 监控指标

**错误日志** (`slog.Error`)：
- `"Sub-agent execution failed"` - 频繁出现可能需要调整重试策略
- `"Failed to update parent session cost"` - 可能表示数据一致性问题

**调试日志** (`slog.Debug`)：
- 重试频率高 → 检查网络稳定性
- 超时频繁 → 增加 timeout 或优化 SubAgent 任务

### 常见问题

#### 1. SubAgent 频繁超时

**症状**：日志中出现大量 `"Sub-agent timed out"`

**排查步骤**：
1. 检查 `subAgentTimeout` 配置
2. 查看日志中的错误类型
3. 优化 SubAgent 的 prompt 或工具集
4. 考虑增加超时时间

**解决方案**：
```go
// 增加超时时间
const subAgentTimeout = 20 * time.Minute
```

#### 2. SubAgent 频繁重试

**症状**：日志中 `"Retrying sub-agent execution"` 频繁出现

**排查步骤**：
1. 检查网络连接稳定性
2. 查看 provider 状态（是否有 API 问题）
3. 检查错误类型是否为可重试错误

**解决方案**：
```go
// 调整重试参数
const (
    subAgentMaxRetries = 5
    subAgentRetryDelay = 3 * time.Second
)
```

#### 3. 错误信息不清晰

**症状**：用户看到的错误消息过于笼统

**排查步骤**：
1. 检查日志中的详细错误
2. 确认 `formatSubAgentError` 处理了所有错误类型
3. 添加新的错误模式匹配

**解决方案**：
```go
// 在 formatSubAgentError 中添加新的错误类型处理
if errors.Is(err, someSpecificError) {
    return "Sub-agent encountered a specific error: " + err.Error()
}
```

### 调试命令

```bash
# 运行 SubAgent 测试
go test -v ./internal/agent/subagent_test.go

# 运行所有 agent 测试（包括竞态检测）
go test -race ./internal/agent/...

# 查看 SubAgent 相关日志
grep -r "Sub-agent" .crush/logs/

# 检查 SubAgent 覆盖率
go test -coverprofile=coverage.out ./internal/agent/...
go tool cover -html=coverage.out
```

## 测试用例

### 测试文件

`internal/agent/subagent_test.go` 包含：

1. **TestSubAgentError** - 测试 SubAgentError 类型
   - 错误消息格式（带/不带会话 ID）
   - Unwrap 方法
   - IsSubAgentError 函数

2. **TestIsRetryableError** - 测试可重试错误检测
   - nil 错误
   - 上下文错误（canceled, deadline exceeded）
   - 网络错误（temporary, timeout）
   - HTTP 错误（5xx, 429, 4xx）
   - 错误消息模式匹配

3. **TestFormatSubAgentError** - 测试错误格式化
   - SubAgentError
   - 上下文错误
   - 通用错误

4. **TestSubAgentTimeoutConstant** - 验证超时常量合理性

5. **TestSubAgentRetryConstants** - 验证重试常量合理性

### 运行测试

```bash
# 运行所有 SubAgent 测试
go test -v ./internal/agent/subagent_test.go

# 生成覆盖率报告
go test -coverprofile=subagent_coverage.out ./internal/agent/subagent_test.go
go tool cover -html=subagent_coverage.out
```

## 并发安全

### 现有机制

- 使用 `context` 进行取消传播
- session 和 message 服务是并发安全的
- 活跃请求跟踪通过 `csync.Map` 实现

### 改进措施

- 超时上下文确保 SubAgent 不会无限期运行
- 重试逻辑中检查上下文取消状态
- 使用 `defer cancel()` 确保超时上下文总是被取消

## 影响范围

### 修改的文件

| 文件 | 作用 |
|------|------|
| `internal/agent/errors.go` | 错误类型定义 |
| `internal/agent/coordinator.go` | runSubAgent 实现 |
| `internal/agent/subagent_test.go` | 单元测试 |
| `internal/agent/agent_tool.go` | agent 工具调用 |
| `internal/agent/agentic_fetch_tool.go` | agentic_fetch 工具调用 |

### 受影响的功能

- ✅ `agent` 工具（SubAgent 调用）
- ✅ `agentic_fetch` 工具（使用 SubAgent 分析网页）
- ✅ 会话成本追踪
- ✅ 错误处理和日志记录

### 向后兼容性

- ✅ API 无破坏性变更
- ✅ 现有功能保持不变
- ✅ 错误处理更加健壮
- ✅ 用户看到更清晰的错误消息

## 相关文档

- `internal/agent/SUBAGENT_FIX.md` - 修复说明
- `internal/agent/SUBAGENT_FIXES.md` - 修复文档
- `docs/reports/subagent_fix_report.md` - 修复总结报告
- `docs/reports/subagent_test_report.md` - 测试报告
- `docs/reports/subagent_sourcegraph_reproduction.md` - 问题复现

## 总结

SubAgent 机制提供了完整的子代理调用功能：

✅ 错误被正确捕获和报告  
✅ 添加了 10 分钟超时控制  
✅ 实现了智能重试机制（网络错误自动重试）  
✅ 改进了错误信息，用户能看到详细错误  
✅ 确保资源正确清理（defer cancel）  
✅ 添加了充分的日志记录  
✅ 并发安全  
✅ 有回退机制（错误时返回友好消息）
