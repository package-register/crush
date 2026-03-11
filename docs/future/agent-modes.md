# Agent 模式切换与自定义 Agent 配置

## 概述

Crush 支持多种 Agent 模式（如 Coder、Task、Plan 等），并允许用户通过外部 TOML 文件定义自定义模式。你可以：

- **动态切换 Agent 模式**：通过命令面板快速切换当前使用的 Agent
- **从目录加载自定义模式**：在指定目录下放置 `*.toml` 文件，即可自动加载并出现在模式列表中
- **自定义 System Prompt 与工具**：为每个模式单独配置提示词和可用工具

## 快速开始

### 1. 打开模式切换对话框

1. 在编辑器中输入 **`/`** 打开命令面板
2. 输入「Switch Agent Mode」或「mode」进行过滤
3. 选择 **Switch Agent Mode** 并回车
4. 在模式列表中选择目标模式并回车完成切换

### 2. 添加自定义 Agent 模式

将 TOML 文件放入默认目录：

```
~/.andy-code/agents/
```

每个 `.toml` 文件对应一个 Agent 模式，文件名（不含扩展名）即模式 ID。例如：

```
~/.andy-code/agents/
├── ask.toml
├── code.toml
├── plan.toml
└── custom_agent.toml
```

重启或重新加载配置后，这些模式会出现在 Switch Agent Mode 对话框中。

## 配置

### Agents 目录

| 优先级 | 配置方式 | 示例 |
|--------|----------|------|
| 1 | 环境变量 `CRUSH_AGENTS_DIR` | `export CRUSH_AGENTS_DIR="/path/to/agents"` |
| 2 | 配置文件 `options.agents_dir` | 见下方 |
| 3 | 默认 | `~/.andy-code/agents` |

在 `crush.json` 中配置：

```json
{
  "options": {
    "agents_dir": "~/.andy-code/agents"
  }
}
```

也可使用绝对路径：

```json
{
  "options": {
    "agents_dir": "/path/to/your/agents"
  }
}
```

目录不存在时，会静默跳过加载，不影响内置模式使用。

### TOML 文件格式

每个 Agent 文件使用顶层字段，支持以下配置：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 显示名称，缺省时使用文件名 |
| `description` | string | 否 | 模式描述，用于列表展示和过滤 |
| `tools` | string[] | 否 | 可用工具列表，空或未填表示全部工具 |
| `system_prompt` | string | 否 | 自定义 System Prompt，覆盖默认模板 |

#### 示例：只读问答模式

```toml
name = "Ask"
description = "只读问答，不修改文件"

tools = ["read_file", "grep", "glob"]
system_prompt = """
你是一个只读助手。
只使用 read_file、grep、glob 了解项目。
不要修改任何文件。
"""
```

#### 示例：完整编辑模式

```toml
name = "Code"
description = "完整代码编辑与工具支持"

tools = ["read_file", "write_file", "grep", "glob", "edit", "list_dir"]
system_prompt = """
你是一个编程助手。
可以使用提供的工具读取、搜索和修改代码。
"""
```

#### 示例：使用全部工具

```toml
name = "Full"
description = "使用所有可用工具"

# 不填写 tools 或留空，表示使用全部工具
```

### 工具名映射

若你熟悉 andy-code 或其他工具的命名，Crush 会将下列名称映射到内置工具：

| andy-code 名称 | Crush 工具 |
|----------------|------------|
| `read_file`    | `view`     |
| `list_dir`     | `ls`       |
| `write_file`   | `write`    |
| `grep`         | `grep`     |
| `glob`         | `glob`     |
| `edit`         | `edit`     |
| `web_fetch`    | `fetch`    |
| `web_search`   | `sourcegraph` |
| `todowrite` / `todoread` | `todos` |

未识别的工具名会被忽略；若映射后无有效工具，将回退为全部工具。

## 容错与健壮性

加载过程中采用容错策略，单个文件或字段错误不会影响其他模式或导致程序崩溃：

| 场景 | 行为 |
|------|------|
| `agents` 目录不存在 | 静默跳过，不影响内置模式 |
| 单个 `.toml` 读取失败 | 跳过该文件，继续处理其余文件 |
| 单个 `.toml` 解析失败 | 跳过并记录日志，不加入模式列表 |
| `name` 缺失 | 使用文件名作为显示名称 |
| `tools` 含未知工具名 | 映射时丢弃，为空则使用全部工具 |
| 选择不存在的 `modeID` | 在切换前校验并提示错误 |

解析失败的文件会输出 `slog.Warn` 日志，便于排查。

## 持久化

切换模式后，所选模式会写入配置：

- 内存中的 `options.active_mode`
- 持久化到 `crush.json` 的 `options.active_mode` 字段

下次启动 Crush 时会自动恢复上次选择的模式。

## 内置模式

除自定义模式外，Crush 内置以下默认模式：

| 模式 ID | 说明 |
|---------|------|
| `coder` | 通用编码助手，工具最全 |
| `task`  | 轻量任务，工具受限 |
| `git`   | Git 相关操作 |
| `rust`  | Rust 开发 |
| `plan`  | 规划与拆解任务 |

自定义模式会与内置模式合并，同名 ID 时自定义模式优先。

## 相关功能

- **Switch Model**（`ctrl+l`）：切换大/小模型，与 Agent 模式独立
- **Sessions**（`ctrl+s`）：管理对话会话
- **Yolo Mode**：跳过权限确认，适合信任环境

## 故障排查

### 模式列表中没有自定义模式

1. 确认 `agents` 目录路径正确（环境变量或 `options.agents_dir`）
2. 确认目录中存在 `.toml` 文件
3. 检查 TOML 语法是否正确
4. 查看日志中是否有 `Skipping agent file` 或 `Failed to read agents directory` 等信息

### 切换模式后无变化

1. 确认 Agent 当前无正在执行的任务（忙碌时会提示等待）
2. 尝试再次执行 Switch Agent Mode 并选择模式

### 工具未按预期工作

1. 检查 `tools` 中使用的工具名是否为支持的 andy-code 名称
2. 确认 `options.disabled_tools` 未禁用所需工具
