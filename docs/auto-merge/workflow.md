# 上游合并流程（Step-by-Step）

本文档记录 2026-03-12 实际执行的完整流程，供下次合并参考。

---

## 步骤 1：检查环境

```bash
cd /path/to/dev/crush

git remote -v
# origin    git@github.com:package-register/crush.git
# upstream  https://github.com/charmbracelet/crush.git

git status
# 位于分支 main
# 您的分支与上游分支 'origin/main' 一致。
# 无文件要提交，干净的工作区
```

## 步骤 2：拉取上游

首次使用 HTTPS 拉取超时，改为 SSH：

```bash
git remote set-url upstream git@github.com:charmbracelet/crush.git
git fetch upstream
```

输出示例：

```
来自 github.com:charmbracelet/crush
 * [新分支]            bump-fang       -> upstream/bump-fang
   ...
   79c3ffa3..f8da538c  main            -> upstream/main
```

## 步骤 3：执行合并

```bash
git merge upstream/main -m "Merge upstream/main: 融合官方 bug 修复与新功能"
```

出现冲突：

```
自动合并 go.mod
自动合并 go.sum
自动合并 internal/agent/agent.go
冲突（内容）：合并冲突于 internal/agent/agent.go
...（共 5 个文件）
```

## 步骤 4：解决冲突

### 4.1 `internal/agent/agent.go`（3 处）

1. **sessionAgent 结构体**：保留 `toolCallFormat` 和 `notify`
2. **SessionAgentOptions**：保留 `ToolCallFormat` 和 `Notify`
3. **NewSessionAgent 初始化**：同时赋值 `toolCallFormat` 和 `notify`

### 4.2 `internal/agent/common_test.go`（1 处）

使用完整 `SessionAgentOptions`，补 `Notify: nil`。

### 4.3 `internal/agent/coordinator.go`（4 处）

1. **常量与错误变量**：保留 `subAgentTimeout` 等常量，加入上游 `err*` 变量
2. **agentCfg fallback**：保留 fallback 逻辑，错误用 `errCoderAgentNotConfigured`
3. **buildAgent**：同时传 `ToolCallFormat` 和 `Notify`
4. **runSubAgent**：保留本地超时与重试逻辑，在 `SessionAgentCall` 中加 `NonInteractive: true`

### 4.4 `internal/config/config.go`（1 处）

Options 合并：保留 `ActiveMode`、`ToolCallFormat`、`AguiServer`、`Bash`，新增 `DisableNotifications`。

### 4.5 `schema.json`（1 处）

在 Options properties 中增加 `disable_notifications`，保留 `agui_server`、`bash`。

## 步骤 5：验证

```bash
go build ./...
go test ./internal/agent/... -count=1
```

## 步骤 6：完成提交

```bash
git add internal/agent/agent.go internal/agent/common_test.go internal/agent/coordinator.go internal/config/config.go schema.json
git status
# 所有冲突已解决但您仍处于合并中。

git commit -m "Merge upstream/main: 融合官方 bug 修复与新功能

- fix(events): panic when metrics disabled
- fix(events): remove redundant posthog exit event
- fix(noninteractive): use models to generate titles
- fix(ui): properly truncate info message
- chore(agent): allocate errors once, reuse errors
- chore(agent): cleanup logic
- feat(notification): alert on turn completion and permission request
- chore(deps): pin fantasy, bump dependencies
- chore: update AGENTS.md
- 保留本地定制: ToolCallFormat, SubAgent 重试/超时, ActiveMode fallback,
  AguiServer, BashOptions, WebDAV 等"
```

## 步骤 7：推送

```bash
git push origin main
# To github.com:package-register/crush.git
#    3e649022..242119ac  main -> main
```

---

## 耗时参考

| 步骤       | 耗时 |
|------------|------|
| fetch      | ~7s  |
| merge      | ~2s  |
| 解决冲突   | 手动 |
| build+test | ~30s |
| 推送       | ~6s  |
