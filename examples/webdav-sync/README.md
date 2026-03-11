# WebDAV 同步示例

本示例展示如何配置和使用 Crush 的 WebDAV 同步功能。

## 目录结构

```
examples/webdav-sync/
├── README.md           # 本文件
├── config-example.json # 配置示例
└── docker-compose.yml  # 测试用 WebDAV 服务器
```

## 快速开始

### 1. 启动测试 WebDAV 服务器

```bash
# 使用 Docker Compose 启动测试服务器
cd examples/webdav-sync
docker-compose up -d

# 或使用 Go 示例服务器
go run ../webdav-server/main.go -port 8080
```

### 2. 配置 WebDAV

创建或编辑 `crush.json`：

```json
{
  "webdav": {
    "enabled": true,
    "url": "http://localhost:8080/crush",
    "username": "admin",
    "password": "admin",
    "remote_path": "/",
    "sync_interval": "5m",
    "conflict_strategy": "newer-wins"
  }
}
```

### 3. 测试连接

```bash
# 测试 WebDAV 服务器
curl -u admin:admin -X PROPFIND http://localhost:8080/crush/

# 预期输出 XML 格式的目录列表
```

### 4. 启动同步

启动 Crush 后，WebDAV 同步会自动开始（如果配置了 sync_interval）。

## 配置示例

### 基本配置 (config-example.json)

```json
{
  "webdav": {
    "enabled": true,
    "url": "https://webdav.example.com/",
    "username": "your-username",
    "password": "${WEBDAV_PASSWORD}",
    "remote_path": "/crush-config",
    "sync_interval": "5m",
    "conflict_strategy": "newer-wins",
    "exclude_patterns": ["*.tmp", "*.lock"],
    "skip_tls_verify": false
  }
}
```

### 使用环境变量

创建 `.env` 文件：

```bash
WEBDAV_URL=https://webdav.example.com/
WEBDAV_USERNAME=admin
WEBDAV_PASSWORD=secret
WEBDAV_REMOTE_PATH=/crush
WEBDAV_SYNC_INTERVAL=5m
WEBDAV_CONFLICT_STRATEGY=newer-wins
```

然后在代码中加载：

```go
import "github.com/charmbracelet/crush/internal/config"

webdavConfig := config.GetSyncConfigFromEnv()
syncManager, _ := config.NewWebDAVSync(webdavConfig, ".crush", logger)
syncManager.Start(ctx)
```

## Docker 测试环境

### docker-compose.yml

```yaml
version: '3'
services:
  webdav:
    image: bytemark/webdav
    ports:
      - "8080:80"
    environment:
      AUTH_TYPE: Digest
      USERNAME: admin
      PASSWORD: admin
    volumes:
      - ./webdav-data:/var/lib/dav
```

### 使用说明

```bash
# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f webdav

# 停止服务
docker-compose down

# 清理数据
docker-compose down -v
```

## Go 示例服务器

### 启动服务器

```bash
go run ../webdav-server/main.go \
  -port 8080 \
  -username admin \
  -password admin \
  -base-path /crush
```

### 测试命令

```bash
# 测试连接
curl -X OPTIONS http://localhost:8080/crush/

# 基本认证测试
curl -u admin:admin -X PROPFIND http://localhost:8080/crush/

# 创建目录
curl -u admin:admin -X MKCOL http://localhost:8080/crush/test/

# 上传文件
curl -u admin:admin -X PUT -d "Hello World" http://localhost:8080/crush/test.txt

# 下载文件
curl -u admin:admin http://localhost:8080/crush/test.txt

# 删除文件
curl -u admin:admin -X DELETE http://localhost:8080/crush/test.txt
```

## 编程示例

### 基本用法

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "time"

    "github.com/charmbracelet/crush/internal/config"
    "github.com/charmbracelet/crush/internal/webdav"
    "github.com/charmbracelet/crush/internal/sync"
)

func main() {
    ctx := context.Background()
    logger := slog.Default()

    // 创建 WebDAV 客户端
    client, err := webdav.NewClient(
        "http://localhost:8080/crush",
        webdav.WithTimeout(30*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }
    client.SetAuth("admin", "admin")

    // 创建同步引擎
    engine, err := sync.NewEngine(sync.Config{
        LocalDir:         ".crush",
        RemotePath:       "/",
        SyncMode:         sync.SyncModeBidirectional,
        ConflictStrategy: sync.ConflictStrategyNewerWins,
        SyncInterval:     5 * time.Minute,
        Logger:           logger,
    }, client)
    if err != nil {
        log.Fatal(err)
    }

    // 启动同步
    if err := engine.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer engine.Stop()

    // 等待同步
    select {}
}
```

### 处理冲突

```go
// 订阅事件
events := engine.SubscribeEvents()
go func() {
    for event := range events {
        switch event.Type {
        case "conflict":
            log.Printf("Conflict detected: %s", event.Path)
            // 自动解决冲突
            if err := engine.ResolveConflict(
                event.Path, 
                sync.ConflictStrategyBackup,
            ); err != nil {
                log.Printf("Failed to resolve conflict: %v", err)
            }
        case "error":
            log.Printf("Error: %v", event.Error)
        }
    }
}()
```

### 手动同步

```go
// 触发手动同步
if err := engine.Sync(); err != nil {
    log.Printf("Sync failed: %v", err)
}

// 获取状态
status := engine.Status()
lastSync := engine.LastSyncTime()
log.Printf("Status: %s, Last sync: %v", status, lastSync)
```

## 故障排查

### 检查连接

```bash
# 测试 WebDAV 服务器
curl -v -u admin:admin -X PROPFIND http://localhost:8080/crush/
```

### 查看日志

```bash
# 启用调试模式
export CRUSH_DEBUG=true
crush
```

### 验证同步

```bash
# 检查本地文件
ls -la .crush/

# 检查远程文件
curl -u admin:admin -X PROPFIND http://localhost:8080/crush/
```

## 最佳实践

1. **使用环境变量存储密码**
   ```bash
   export WEBDAV_PASSWORD="secret"
   ```

2. **配置合适的同步间隔**
   ```json
   "sync_interval": "5m"
   ```

3. **排除临时文件**
   ```json
   "exclude_patterns": ["*.tmp", "*.lock", ".sync_state.json"]
   ```

4. **使用备份策略**
   ```json
   "conflict_strategy": "backup"
   ```

5. **定期验证同步状态**
   ```bash
   # 检查同步日志
   tail -f crush.log | grep webdav
   ```

## 相关资源

- [配置指南](../../docs/webdav/configuration.md)
- [API 文档](../../docs/webdav/api.md)
- [故障排查](../../docs/webdav/troubleshooting.md)
- [WebDAV 服务器示例](../webdav-server/)
