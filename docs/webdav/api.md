# WebDAV API 文档

## 包说明

```go
import "github.com/charmbracelet/crush/internal/webdav"
import "github.com/charmbracelet/crush/internal/sync"
import "github.com/charmbracelet/crush/internal/config"
```

## WebDAV Client

### 创建客户端

```go
// 基本用法
client, err := webdav.NewClient("https://webdav.example.com/")

// 带配置
client, err := webdav.NewClient(
    "https://webdav.example.com/",
    webdav.WithTimeout(30*time.Second),
    webdav.WithRetryConfig(5, 100*time.Millisecond),
    webdav.WithSkipTLSVerify(false),
    webdav.WithHeaders(map[string]string{
        "X-Custom-Header": "value",
    }),
)
```

### 认证

```go
// 基本认证
client.SetAuth("username", "password")

// Token 认证
client.SetToken("bearer-token")
```

### 基本操作

```go
ctx := context.Background()

// 测试连接
err := client.Ping(ctx)

// 下载文件
data, err := client.Get(ctx, "/path/to/file.txt")

// 下载文件（带 ETag）
data, etag, err := client.GetWithETag(ctx, "/path/to/file.txt")

// 上传文件
err := client.Put(ctx, "/path/to/file.txt", data)

// 条件上传（仅当文件不存在时）
err := client.PutIfNoneMatch(ctx, "/path/to/file.txt", data)

// 条件上传（仅当 ETag 匹配时）
err := client.PutIfMatch(ctx, "/path/to/file.txt", data, etag)

// 删除文件
err := client.Delete(ctx, "/path/to/file.txt")

// 创建目录
err := client.MkCol(ctx, "/path/to/dir/")

// 复制文件/目录
err := client.Copy(ctx, "/src", "/dst", true)

// 移动文件/目录
err := client.Move(ctx, "/src", "/dst", true)
```

### PROPFIND / PROPPATCH

```go
// PROPFIND 查询属性
propFindXML := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
    <D:prop>
        <D:resourcetype/>
        <D:displayname/>
        <D:getcontenttype/>
        <D:getcontentlength/>
        <D:getetag/>
        <D:getlastmodified/>
    </D:prop>
</D:propfind>`

resp, err := client.PropFind(ctx, "/path/", 1, propFindXML)
for _, r := range resp.Responses {
    info := r.ToResourceInfo()
    fmt.Printf("Path: %s, Size: %d, ETag: %s\n", 
        info.Path, info.Size, info.ETag)
}

// PROPPATCH 更新属性
propPatchXML := `<?xml version="1.0" encoding="utf-8"?>
<D:proppatch xmlns:D="DAV:">
    <D:set>
        <D:prop>
            <D:displayname>New Name</D:displayname>
        </D:prop>
    </D:set>
</D:proppatch>`

err := client.PropPatch(ctx, "/path/to/file.txt", propPatchXML)
```

### 锁定

```go
// 创建锁
lockToken, err := client.Lock(ctx, "/path/to/file.txt", "Infinite", "Crush Sync")

// 释放锁
err := client.Unlock(ctx, "/path/to/file.txt", lockToken)
```

## Sync Engine

### 创建同步引擎

```go
import "github.com/charmbracelet/crush/internal/sync"

engine, err := sync.NewEngine(sync.Config{
    LocalDir:         "/path/to/.crush",
    RemotePath:       "/crush-config",
    SyncMode:         sync.SyncModeBidirectional,
    ConflictStrategy: sync.ConflictStrategyNewerWins,
    ExcludePatterns:  []string{"*.tmp", "*.lock"},
    SyncInterval:     5 * time.Minute,
    Logger:           logger,
}, client)
```

### 控制同步

```go
ctx := context.Background()

// 启动引擎
err := engine.Start(ctx)

// 手动同步
err := engine.Sync()

// 停止引擎
err := engine.Stop()

// 获取状态
status := engine.Status()
lastSync := engine.LastSyncTime()
```

### 处理冲突

```go
// 订阅事件
events := engine.SubscribeEvents()
go func() {
    for event := range events {
        switch event.Type {
        case "conflict":
            fmt.Printf("Conflict detected: %s\n", event.Path)
        case "error":
            fmt.Printf("Error: %v\n", event.Error)
        }
    }
}()

// 解决冲突
err := engine.ResolveConflict("/path/to/file.txt", sync.ConflictStrategyBackup)
```

### 同步模式

```go
// 双向同步（默认）
sync.SyncModeBidirectional

// 仅上传
sync.SyncModeUpload

// 仅下载
sync.SyncModeDownload
```

### 冲突策略

```go
// 新版本获胜
sync.ConflictStrategyNewerWins

// 本地获胜
sync.ConflictStrategyLocalWins

// 远程获胜
sync.ConflictStrategyRemoteWins

// 创建备份
sync.ConflictStrategyBackup

// 手动解决
sync.ConflictStrategyManual
```

## Config 集成

### 创建 WebDAV 同步管理器

```go
import "github.com/charmbracelet/crush/internal/config"

webdavConfig := &config.WebDAVConfig{
    Enabled:          true,
    URL:              "https://webdav.example.com/",
    Username:         "username",
    Password:         "${WEBDAV_PASSWORD}",
    RemotePath:       "/crush-config",
    SyncInterval:     "5m",
    ConflictStrategy: "newer-wins",
}

syncManager, err := config.NewWebDAVSync(webdavConfig, "/path/to/.crush", logger)

// 启动同步
err = syncManager.Start(ctx)

// 手动同步
err = syncManager.Sync()

// 获取状态
status := syncManager.Status()
lastSync := syncManager.LastSyncTime()

// 停止同步
err = syncManager.Stop()
```

### 从环境变量加载配置

```go
webdavConfig := config.GetSyncConfigFromEnv()
// 需要设置以下环境变量：
// - WEBDAV_URL
// - WEBDAV_USERNAME
// - WEBDAV_PASSWORD
// - WEBDAV_REMOTE_PATH
// - WEBDAV_SYNC_INTERVAL (可选)
// - WEBDAV_CONFLICT_STRATEGY (可选)
```

## 错误处理

### WebDAV 错误类型

```go
// 检查错误类型
var wdErr *webdav.Error
if errors.As(err, &wdErr) {
    if wdErr.IsRetryable() {
        // 可以重试的错误
    }
    if wdErr.IsConflict() {
        // 冲突错误
    }
}

// 常见错误
webdav.ErrUnauthorized      // 401 未授权
webdav.ErrNotFound          // 404 未找到
webdav.ErrConflict          // 409 冲突
webdav.ErrForbidden         // 403 禁止
webdav.ErrTimeout           // 超时
webdav.ErrConnectionFailed  // 连接失败
```

### 重试逻辑

```go
client, _ := webdav.NewClient(
    url,
    webdav.WithRetryConfig(5, 100*time.Millisecond),
)

// 自动重试可恢复的错误
// - 网络错误
// - 5xx 服务器错误
// - 429 请求过多
// - 408/504 超时
```

## 事件类型

| 事件类型 | 说明 |
|---------|------|
| `upload` | 文件已上传 |
| `download` | 文件已下载 |
| `delete` | 文件已删除 |
| `conflict` | 检测到冲突 |
| `error` | 发生错误 |
| `conflict_resolved` | 冲突已解决 |

## 最佳实践

1. **使用上下文控制超时**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   err := client.Get(ctx, path)
   ```

2. **处理冲突事件**
   ```go
   events := engine.SubscribeEvents()
   for event := range events {
       if event.Type == "conflict" {
           // 处理冲突
       }
   }
   ```

3. **优雅停止**
   ```go
   defer engine.Stop()
   ```

4. **错误日志记录**
   ```go
   config.Logger = slog.Default()
   ```
