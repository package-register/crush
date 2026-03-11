# 上游合并指南

本目录记录从 `charmbracelet/crush` 融合官方变更的流程与策略，方便后续复用。

## 一、快速流程（TL;DR）

```bash
cd /path/to/dev/crush

# 1. 确保 upstream 已配置（若用 HTTPS 慢，可改用 SSH）
git remote -v
git remote set-url upstream git@github.com:charmbracelet/crush.git   # 可选

# 2. 拉取并合并
git fetch upstream
git merge upstream/main -m "Merge upstream/main: 融合官方 bug 修复与新功能"

# 3. 解决冲突 → 见下文「冲突解决策略」
# 4. 验证并推送
go build ./...
go test ./internal/agent/...
git add <已解决的文件>
git commit -m "Merge upstream/main: ..."
git push origin main
```

---

## 二、详细流程

### 2.1 前置检查

| 项     | 说明 |
|--------|------|
| 工作区 | 确保 `git status` 干净，无未提交改动 |
| 远程   | `origin` → package-register/crush，`upstream` → charmbracelet/crush |
| 网络   | 若 `git fetch upstream` 超时，改用 SSH：`git remote set-url upstream git@github.com:charmbracelet/crush.git` |

### 2.2 执行合并

```bash
git fetch upstream
git merge upstream/main -m "Merge upstream/main: 融合官方 bug 修复与新功能"
```

### 2.3 常见冲突文件及处理思路

| 文件 | 典型冲突 | 处理原则 |
|------|----------|----------|
| `internal/agent/agent.go` | `toolCallFormat` vs `notify` | 二者都要，同时保留 |
| `internal/agent/common_test.go` | `SessionAgentOptions` 字段差异 | 保留本地字段，补上上游新增字段（如 `Notify: nil`） |
| `internal/agent/coordinator.go` | 常量 vs 错误变量、SubAgent 逻辑 | 上游错误变量 + 本地 SubAgent 超时/重试逻辑 |
| `internal/config/config.go` | Options 字段（本地扩展 vs 上游新增） | 合并两边字段 |
| `schema.json` | Options 属性 | 合并两边属性 |

### 2.4 验证

```bash
go build ./...
go test ./internal/agent/... -count=1
```

### 2.5 完成合并

```bash
git add <已解决冲突的文件>
git status   # 确认「所有冲突已解决」
git commit   # 若上一步未带 -m，会弹出编辑器填写
git push origin main
```

---

## 三、冲突解决策略（本次实践）

### 3.1 `agent.go`

- **冲突**：`sessionAgent` 结构体、`SessionAgentOptions` 中 `toolCallFormat` vs `notify`
- **策略**：二者都保留，上游 `notify` 用于回合完成通知，本地 `toolCallFormat` 用于 longcat 模式

### 3.2 `coordinator.go`

- **常量 vs 错误变量**：保留本地 `subAgentTimeout` 等常量，同时引入上游 `errCoderAgentNotConfigured` 等命名错误
- **ActiveMode fallback**：保持「优先 coder，再 fallback」逻辑，但用 `errCoderAgentNotConfigured` 替代裸字符串错误
- **buildAgent**：同时传入 `ToolCallFormat` 和 `Notify`
- **runSubAgent**：保留本地超时、重试逻辑，在 `SessionAgentCall` 中增加 `NonInteractive: true`（与上游保持一致）

### 3.3 `config.go` 与 `schema.json`

- **Options**：本地字段（`ActiveMode`、`ToolCallFormat`、`AguiServer`、`Bash`）与上游字段（`DisableNotifications`）全部保留
- **schema.json**：在 Options 的 properties 中同时包含 `agui_server`、`bash`、`disable_notifications`

### 3.4 `common_test.go`

- **SessionAgentOptions**：使用完整字段，包括 `SystemPromptPrefix`、`IsSubAgent`、`DisableAutoSummarize`、`ToolCallFormat`，并添加 `Notify: nil`

---

## 四、本地定制清单（合并时勿覆盖）

合并时需确保以下定制被保留：

| 类别 | 内容 |
|------|------|
| 配置 | `ActiveMode` fallback、`ToolCallFormat`、`AguiServer`、`BashOptions`、WebDAV |
| Agent | longcat 解析、SubAgent 超时与重试 |
| 更新 | `GitHubRepo` 指向 `package-register/crush`、自定义 User-Agent |
| CI/CD | `release-simple.yml` 并行构建、ldflags 注入版本等 |

---

## 五、参考信息

- 上游仓库：https://github.com/charmbracelet/crush
- 本 fork：https://github.com/package-register/crush
- 上次合并时间：2026-03-12
- 上次合并提交：`242119ac`（Merge upstream/main: 融合官方 bug 修复与新功能）
