---
trigger: manual
---

# Go 项目自助技术选型与实现工作流

## 概述
这是一份给 LLM 的工作流指南，适配 Windsurf 规则。当用户提供 Go 项目需求时，LLM 应自动完成技术选型、文档查询、代码生成和验证的全流程，无需用户逐步指导。

---

## 核心原则

### 自主执行
- 不询问用户选择哪个库，自己决策
- 不询问 API 用法，自己查文档
- 遇到问题自动尝试多种解决方案
- 编译失败自动修复，不等待用户反馈
- 完整实现功能，不留 TODO 或占位符

### 文档优先
- 所有代码必须基于官方文档或示例
- 从 pkg.go.dev、GitHub 官方示例获取真实 API
- 不臆想 API，不编造版本号
- 遇到不确定的 API，先查文档再写代码

### 快速反馈
- 每个步骤完成后立即验证（编译、测试）
- 不要生成大量代码后才验证
- 错误立即修复，不积累问题
- 遵循 Windsurf 规则：READ BEFORE EDITING、BE AUTONOMOUS、TEST AFTER CHANGES

---

## 工作流程

### 第一步：需求理解与库选型

**输入**：用户需求 + 可选的库建议

**执行**：
1. 解析用户需求的核心功能
2. 如果用户提供了库建议，优先考虑
3. 如果没有建议，自动搜索候选库：
   - 用 `fetch` 工具访问 pkg.go.dev 查询库信息
   - 检查 GitHub Stars、最后更新时间、文档完整度
   - 对比 2-3 个候选库的优劣
4. 输出选型决策（简述理由）

**输出示例**：
```
选择库：github.com/gin-gonic/gin v1.x（Web 框架）+ gorm.io/gorm v1.x（ORM）
理由：gin 是 Go Web 开发标准，GORM 是最活跃的 ORM，文档完整
```

---

### 第二步：文档获取与 API 学习

**执行**：
1. 运行 `go mod vendor` 获取源码级别的依赖
2. 用 `go doc [包名]` 查询本地官方文档（最快、最准确）
3. 用 `rg` 在 vendor/ 中搜索 API 用法示例
4. 用 `fd` 找 examples/ 或 _examples/ 目录中的示例代码
5. 必要时用 `fetch` 从 pkg.go.dev 获取补充文档
6. 检查 go.mod 中的版本要求

**关键信息**：
- 函数/结构体/接口签名（通过 `go doc` 获取）
- 常见初始化方式（通过 vendor 源码查看）
- 错误处理模式（通过 `rg` 搜索示例）
- 官方示例代码（通过 `fd` 找 examples/）
- 依赖版本兼容性（通过 go.mod 确认）

**执行示例**：
```bash
# 查询 gin 的 Engine 类型
go doc github.com/gin-gonic/gin.Engine

# 在 vendor 中搜索 Handler 的使用方式
rg "func.*Handler" vendor/github.com/gin-gonic/gin/

# 找 gin 的示例文件
fd "example.*\.go$" vendor/github.com/gin-gonic/gin/

# 查看源码级别的实现
view vendor/github.com/gin-gonic/gin/gin.go
```

---

### 第三步：项目初始化

**执行**：
1. 生成 go.mod（包含精确版本号）
2. 创建项目目录结构（遵循 Go 标准布局）
3. 基于官方示例生成初始代码
4. 确保代码与文档中的 API 完全一致
5. 添加必要的错误处理和日志

**标准目录结构**：
```
project/
├── go.mod
├── go.sum
├── main.go
├── cmd/
│   └── app/
│       └── main.go
├── internal/
│   ├── handler/
│   ├── service/
│   └── model/
├── pkg/
│   └── util/
└── README.md
```

**输出示例**：
```
生成文件：
  - go.mod
  - go.sum
  - cmd/app/main.go
  - internal/handler/handler.go
  - internal/service/service.go

核心 API：
  - gin.Default() - 创建路由引擎
  - gorm.Open() - 连接数据库
```

---

### 第四步：编译验证

**执行**：
1. 运行 `go mod tidy` 整理依赖
2. 运行 `go build ./cmd/app` 验证编译
3. 如果编译失败：
   - 读取完整错误信息
   - 查阅相关文档重新调整代码
   - 重新运行 `go build` 直到成功
4. 如果编译成功，运行 `go test ./...` 运行测试

**输出示例**：
```
✓ go mod tidy 成功
✓ go build 成功
✓ go test 通过
```

或

```
✗ go build 失败：undefined: someFunc
→ 查阅文档发现正确 API 是 SomeFunc()
→ 修复代码...
✓ go build 成功
```

---

### 第五步：反馈与迭代

**场景 1：编译错误**
- 自动读取错误信息
- 查文档找到正确 API
- 修改代码
- 重新验证

**场景 2：功能不符**
- 用户描述问题
- LLM 查文档理解需求
- 修改代码
- 重新验证

**场景 3：性能或设计问题**
- 用户提出改进建议
- LLM 查文档找到最佳实践
- 重构代码
- 重新验证

重复直到用户满意。

---

## 工具使用指南

| 工具 | 用途 | 示例 |
|------|------|------|
| `bash` + `go doc` | 本地官方文档查询 | `go doc gin.Engine` 查看 API 签名 |
| `bash` + `rg` | 快速代码搜索 | `rg "func.*Handler" vendor/` 找 API 用法 |
| `bash` + `fd` | 精确文件查找 | `fd "example.*\.go$" vendor/` 找示例代码 |
| `bash` + `go mod vendor` | 获取源码级别依赖 | 本地查看库源码和示例 |
| `fetch` | 在线文档补充 | pkg.go.dev 获取完整文档 |
| `view` | 查看本地文件 | 检查生成的代码或 vendor 源码 |
| `edit` | 修改本地文件 | 修复编译错误 |
| `lsp_diagnostics` | 检查代码问题 | 验证语法和类型错误 |

---

## Windsurf 规则适配

### READ BEFORE EDITING
- 使用 `view` 工具读取文件内容
- 注意精确的缩进、空格、换行符
- 复制完整的上下文（3-5 行）用于编辑
- 验证 old_string 在文件中唯一出现

### BE AUTONOMOUS
- 不询问用户，自动搜索和决策
- 尝试多种解决方案直到成功
- 只在真正阻塞时才停止
- 完整实现功能，不留占位符

### TEST AFTER CHANGES
- 每次修改后立即运行 `go build` 或 `go test`
- 使用 `lsp_diagnostics` 检查类型错误
- 验证编译成功后再进行下一步
- 不积累多个未验证的修改

### BE CONCISE
- 输出保持在 4 行以内（工具调用不计）
- 不重复说明已执行的操作
- 直接给出结果，不需要过程描述
- 只在必要时解释复杂决策

### USE EXACT MATCHES
- 编辑时包含完整的上下文
- 匹配精确的缩进（制表符 vs 空格）
- 包含所有空行和换行符
- 验证文本在文件中唯一

---

## 输出格式规范

### 选型阶段
```
选择库：[库名] v[版本]
理由：[简述为什么选这个]
```

### 代码生成阶段
```
生成文件：[文件列表]
核心 API：[关键函数/结构体]
```

### 验证阶段
```
✓ go mod tidy 成功
✓ go build 成功
✓ go test 通过
```

或

```
✗ go build 失败：[错误] → 修复中...
✓ go build 成功
```

---

## 用户交互模式

### 用户输入示例
- "用 gin 写个 REST API 服务器"
- "我需要一个数据库 ORM，推荐 GORM"
- "编译失败了：undefined: Handler"
- "这个功能不对，应该是..."

### LLM 响应原则
- 不要长篇大论解释
- 直接执行工作流
- 只在关键决策点简述理由
- 最后输出可用的项目代码
- 保持输出简洁（<4 行文本，除非必要）

---

## 成功标准

✓ 项目能编译通过（`go build` 成功）
✓ 代码基于官方文档
✓ 用户只需给方向，其余全自动
✓ 没有幻觉 API 或不存在的版本
✓ 首次生成就能运行（不需要多轮修复）
✓ 遵循 Go 标准项目布局
✓ 包含必要的错误处理

---

## 常见场景处理

### 场景 1：用户只给需求，不指定库
```
用户：写个 Web API
LLM：
  1. 搜索候选库（gin, echo, fiber）
  2. 对比文档和 GitHub Stars
  3. 选择 gin（最活跃）
  4. 从 pkg.go.dev 获取示例
  5. 生成项目
  6. go build 验证
```

### 场景 2：用户指定库但版本不明确
```
用户：用 GORM 做数据库操作
LLM：
  1. 查 pkg.go.dev 获取最新稳定版本
  2. 从 GitHub 获取 GORM 示例
  3. 生成 go.mod（精确版本号）
  4. 生成代码
  5. go build 验证
```

### 场景 3：编译失败
```
用户：编译失败了
LLM：
  1. 读取完整错误信息
  2. 查文档找到正确 API
  3. 修改代码
  4. go build 验证
  5. 输出修复结果
```

### 场景 4：功能需求变更
```
用户：需要支持 X 功能
LLM：
  1. 查文档找到相关 API
  2. 修改代码添加功能
  3. go build 验证
  4. go test 验证
  5. 输出更新结果
```

---

## 禁止事项

❌ 不要臆想 API 签名
❌ 不要编造库版本号
❌ 不要生成大量代码后才验证
❌ 不要等待用户逐步指导
❌ 不要在代码中留下 TODO 或未实现的部分
❌ 不要使用不存在的依赖
❌ 不要编辑未读过的文件
❌ 不要忽略编译错误

---

## 快速参考

**技术选型渠道**：
- pkg.go.dev - 库信息、文档、版本
- GitHub - examples/ 目录、README、Stars
- go.dev - 官方 Go 文档

**验证命令**：
- `go mod tidy` - 整理依赖
- `go build ./cmd/app` - 编译检查
- `go test ./...` - 运行测试
- `go fmt ./...` - 代码格式化
- `go vet ./...` - 代码检查

**文档查询**：
- `go doc [包名]` - 本地 API 文档（最准确、最快）
- `go doc [包名].[类型]` - 查询具体类型
- `rg "[模式]" vendor/` - 在源码中搜索用法
- `fd "[模式]" vendor/` - 找示例或特定文件
- `go mod vendor` - 获取源码级别依赖
- pkg.go.dev/[库名] - 在线文档（补充）

**Go 标准布局**：
- cmd/ - 可执行程序入口
- internal/ - 内部包（不可导出）
- pkg/ - 公共包（可导出）
- test/ - 测试文件

---

## 总结

这个工作流的目标是让 LLM 成为一个自主的 Go 项目初始化助手：

1. **用户给方向** → LLM 自动选型
2. **LLM 查文档** → 获取真实 API
3. **LLM 生成代码** → 基于官方示例
4. **LLM 验证** → 编译、测试、修复
5. **用户得到可用项目** → 无需多轮修复

关键是**自主执行**、**文档优先**和**立即验证**，避免幻觉和臆想。遵循 Windsurf 规则确保代码质量和执行效率。
