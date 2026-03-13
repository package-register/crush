# AGUI 配置对话框改进 - 验证报告 (C1–C5)

## C1 单元测试覆盖

| 包/组件 | 覆盖率 | 说明 |
|---------|--------|------|
| `internal/app` | 40.8% | `StartOrRestartAguiServer` 84.6% |
| `internal/ui/dialog` | 3.7% | `NewAguiConfig`、`buildConfigFromInputs` 100%，`HandleMsg` 4.3% |

**新增测试**：
- `TestStartOrRestartAguiServer`：启用时启动 AGUI
- `TestStartOrRestartAguiServerDisabled`：禁用时 no-op
- `TestNewAguiConfig`：初始化与 ID
- `TestNewAguiConfigWithOptions`：带 options 初始化
- `TestAguiConfigHandleMsgCancel`：ESC 关闭
- `TestAguiConfigBuildConfigFromInputs`：buildConfigFromInputs 逻辑

---

## C2 全项目测试

- **结果**：通过
- **命令**：`go test ./...`
- 所有含测试的包均通过，无回归

---

## C3 影响范围分析

### Action 传播

| Action | 处理位置 | 状态 |
|--------|----------|------|
| `ActionConfirmStartAGUI` | `ui.go` handleDialogMsg 第 1227 行 | ✓ 已处理 |
| 无其他 switch action 遗漏 | 仅 handleDialogMsg 处理 | ✓ 已确认 |

### Dialog 栈

- `ActionConfirmStartAGUI` 触发 `CloseFrontDialog()`，与 `ActionClose` 一致
- 确认弹窗为 AguiConfig 内部状态 (`confirmingStart`)，非新 dialog，无栈问题

### Config 写入

- `saveConfig(true)` 调用 `cfg.SetConfigField("options.agui_server", aguiCfg)`
- 无其他 listener 或热重载依赖此字段
- **结论**：已确认无影响

### AGUI 生命周期

- `StartOrRestartAguiServer`：先 `Stop` 再 `NewServer` + `Start`
- `app.AguiServer` 替换期间，原 server 已 Stop，无并发访问
- cleanupFuncs 追加新的 Stop，Shutdown 时正确清理
- **结论**：已确认无影响

---

## C4 资源竞争（Race）检测

- **结果**：通过
- **命令**：`go test -race ./internal/ui/... ./internal/app/... ./internal/agui-server/...`
- 无 race 报告

---

## C5 集成/手工回归 Checklist

请在真实环境中逐项验证：

- [ ] AGUI 配置对话框：标签在上、输入框在下（分行布局）
- [ ] Tab / ↓ 切换到下一个输入框
- [ ] Shift+Tab / ↑ 切换到上一个输入框
- [ ] 聚焦在 CORS Origins（最后一个）按 Enter：弹出「Start AGUI server now?」
- [ ] 选「启动」：保存、关闭、AGUI 启动，状态栏显示 "AGUI server started"
- [ ] 选「取消」：返回表单
- [ ] ctrl+s 保存：关闭对话框（不弹确认）
- [ ] WebDAV 配置：标签与输入分行布局
- [ ] 关闭对话框后焦点回到编辑器
- [ ] 启用 AGUI 后，curl `http://localhost:{port}/agui/...` 可访问
