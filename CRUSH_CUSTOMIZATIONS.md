# Crush 定制变更记录

本文档记录本地 fork 相对于官方 [charmbracelet/crush](https://github.com/charmbracelet/crush) 的所有定制修改，便于：

1. **追踪变更**：清楚知道改了什么
2. **合并上游**：合并官方新功能/修复时，能快速定位冲突点
3. **冲突解决**：合并时参考本文档，保留定制逻辑

---

## 变更清单总览

| # | 功能 | 涉及文件 | 合并时注意 |
|---|------|----------|------------|
| 1 | 自定义模式（modes.toml） | config/modes.go, config/config.go, config/load.go | 新增文件 + 配置扩展 |
| 2 | MCP 热重载 | agent/tools/mcp/init.go | 新增 Reload() |
| 3 | Longcat ToolCall 解析 | agent/longcat.go, agent/agent.go | 新增文件 + agent 集成 |
| 4 | 光标样式（块→细线） | ui/styles/styles.go | 单行修改 |
| 5 | 自定义命令示例 | .crush/commands/*.md | 项目级，可忽略 |
| 6 | exit/quit 等效 | ui/dialog/commands_item.go, commands.go | aliases 参数 |
| 7 | 双套快捷键（IDE vs 独立终端） | config, keys.go, ui.go, completions, quit.go | 多处 |
| 8 | Shift+Enter 换行 | ui/model/keys.go, ui.go | 键绑定 + 处理顺序 |

---

## 1. 自定义模式（modes.toml）

### 文件

- **新增**：`internal/config/modes.go`（完整文件）
- **修改**：`internal/config/config.go`（Options.ActiveMode、模式加载逻辑）
- **修改**：`internal/config/load.go`（加载 modes.toml）
- **示例**：`modes.toml.example`

### 功能

- 从 `modes.toml` 动态加载模式（Git、Rust、Plan 等）
- 查找顺序：全局 → 项目（CWD 向上）
- 支持 `allowed_tools`、`allowed_mcp`、`context_paths` 等

### 合并注意

- 若官方新增 `modes.go` 或类似功能，需对比合并
- `config.go` 中 `ActiveMode` 的默认值、`load.go` 中的加载逻辑可能冲突

---

## 2. MCP 热重载

### 文件

- **修改**：`internal/agent/tools/mcp/init.go`

### 功能

- 新增 `Reload(ctx, permissions, cfg)`：关闭所有 MCP 连接，按当前配置重新初始化
- 使用 `sessions.Reset()`、`allTools.Reset()` 等清空状态
- 命令面板中 "Reload MCP" 触发

### 合并注意

- 若官方增加 MCP 相关 API，需确保 `Reload` 与新逻辑兼容
- `csync.Map` 的 `Reset` 用法可能随依赖变化

---

## 3. Longcat ToolCall 解析

### 文件

- **新增**：`internal/agent/longcat.go`（完整文件）
- **修改**：`internal/agent/agent.go`（集成 ParseLongcatToolCalls、HasIncompleteLongcatToolCall）

### 功能

- 解析 `<longcat_tool_call>{"name":"...","arguments":{...}}</longcat_tool_call>` 格式
- 支持流式输出中未闭合的标签
- 配置：`options.tool_call_format` = `"longcat"` 时启用

### 合并注意

- `SessionAgentOptions` 中 `ToolCallFormat` 字段
- `coordinator.go`、`agentic_fetch_tool.go` 中构造 `SessionAgentOptions` 时需传入该字段
- 若官方增加其他 tool call 格式，可参考本实现扩展

---

## 4. 光标样式（块状 → 细线）

### 文件

- **修改**：`internal/ui/styles/styles.go`

### 功能

- `tea.CursorBlock` → `tea.CursorBar`（细竖线 I-beam）
- 影响：TextInput、TextArea 的 Cursor.Shape

### 合并注意

- 单行修改，冲突概率低
- 若官方增加主题/光标配置，可考虑迁移到配置项

---

## 5. 自定义命令示例

### 文件

- `.crush/commands/review.md`
- `.crush/commands/explain.md`

### 功能

- 示例命令，支持 `$FOCUS` 等参数
- 项目级配置，不影响核心代码

### 合并注意

- 可忽略，通常不会冲突

---

## 6. exit / quit 等效

### 文件

- **修改**：`internal/ui/dialog/commands_item.go`（NewCommandItem 增加 `aliases` 参数）
- **修改**：`internal/ui/dialog/commands.go`（Quit 命令传入 `aliases: "exit", "quit"`）

### 功能

- 命令面板中输入 "exit" 或 "quit" 均可匹配 Quit 命令

### 合并注意

- `NewCommandItem` 签名变化：新增可选 `aliases ...string`
- 其他调用 `NewCommandItem` 的地方需补全参数（可传空）

---

## 7. 双套快捷键（IDE vs 独立终端）

### 文件

- **修改**：`internal/config/config.go`
  - `TUIOptions.KeybindingScheme`（auto | ide | standalone）
  - `EffectiveKeybindingScheme()`：auto 时根据 `TERM_PROGRAM == "vscode"` 判断
- **修改**：`internal/ui/model/keys.go`
  - `DefaultKeyMapForScheme(scheme)`、`defaultKeyMapIDE()`、`defaultKeyMapStandalone()`
- **修改**：`internal/ui/model/ui.go`（按 scheme 选择 KeyMap）
- **修改**：`internal/ui/completions/keys.go`（按 scheme 使用 alt/ctrl）
- **修改**：`internal/ui/dialog/quit.go`（按 scheme 显示 alt+q 或 ctrl+c）
- **新增**：`internal/config/keybinding_test.go`

### 功能

- **ide**：Alt 系（alt+q 退出、alt+p 命令、alt+m 模型等），减少 VS Code/Cursor 内嵌终端冲突
- **standalone**：Ctrl 系（ctrl+c 退出、ctrl+p 命令等），独立终端习惯
- **auto**：根据 `TERM_PROGRAM == "vscode"` 自动选择

### 合并注意

- 若官方实现 [crush#737](https://github.com/charmbracelet/crush/issues/737) 可配置 keybindings，可考虑迁移
- `keys.go` 中两套 KeyMap 逻辑较多，合并时需仔细对比

---

## 8. Shift+Enter 换行

### 文件

- **修改**：`internal/ui/model/keys.go`
  - Editor.Newline 增加 `shift+enter`、`shift+return` 绑定
- **修改**：`internal/ui/model/ui.go`
  - 将 Newline 的检查放在 SendMessage 之前（避免 shift+enter 被误当作发送）

### 功能

- 支持 Shift+Enter / Shift+Return 换行
- 部分终端报告 "shift+return" 而非 "shift+enter"

### 合并注意

- 键绑定与处理顺序，可能与官方输入处理逻辑冲突

---

## 合并上游的推荐流程

1. **备份**：`git stash` 或提交当前定制到 `custom` 分支
2. **添加上游**（若尚未添加）：
   ```bash
   git remote add upstream https://github.com/charmbracelet/crush.git
   git fetch upstream
   ```
3. **合并**：
   ```bash
   git merge upstream/main
   ```
4. **解决冲突**：参考本文档，逐项检查冲突文件，保留定制逻辑
5. **验证**：
   ```bash
   go build ./...
   go test ./...
   ```

---

## 相关文档

- `resources/summary/crush-implementation-experience.md` - 实现经验
- `resources/summary/crush-cursor-customization-experience.md` - 光标定制
- `resources/crush-vscode-shortcut-conflict-analysis.md` - 快捷键冲突分析

---

*最后更新：2025-03-08*
