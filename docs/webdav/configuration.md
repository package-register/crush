# WebDAV 同步配置指南

## 概述

Crush 支持通过 WebDAV 协议同步配置文件，方便在多台设备之间保持配置一致。支持的主流 WebDAV 服务包括：

- Nextcloud
- ownCloud
- Seafile
- 标准 WebDAV 服务器

## 快速开始

### 1. 基本配置

在 `crush.json` 中添加 WebDAV 配置：

```json
{
  "webdav": {
    "enabled": true,
    "url": "https://webdav.example.com/",
    "username": "your-username",
    "password": "${WEBDAV_PASSWORD}",
    "remote_path": "/crush-config",
    "sync_interval": "5m",
    "conflict_strategy": "newer-wins"
  }
}
```

### 2. 使用环境变量

推荐将敏感信息存储在环境变量中：

```bash
export WEBDAV_PASSWORD="your-secret-password"
export WEBDAV_TOKEN="your-bearer-token"
```

然后在配置中引用：

```json
{
  "webdav": {
    "enabled": true,
    "url": "${WEBDAV_URL}",
    "password": "${WEBDAV_PASSWORD}",
    "remote_path": "/crush-config"
  }
}
```

## 配置选项

### 必需选项

| 选项 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `enabled` | boolean | 是否启用 WebDAV 同步 | `true` |
| `url` | string | WebDAV 服务器 URL | `https://webdav.example.com/` |

### 认证选项

| 选项 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `username` | string | 用户名 | `admin` |
| `password` | string | 密码（支持环境变量） | `${WEBDAV_PASSWORD}` |
| `token` | string | Bearer Token（可选） | `abc123...` |

### 同步选项

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `remote_path` | string | `/` | 远程目录路径 |
| `sync_interval` | string | `""` | 同步间隔（如 `5m`, `1h`），空表示仅手动同步 |
| `conflict_strategy` | string | `newer-wins` | 冲突解决策略 |
| `exclude_patterns` | array | `[]` | 排除的文件模式 |
| `skip_tls_verify` | boolean | `false` | 跳过 TLS 验证（仅测试用） |

### 冲突解决策略

| 策略 | 说明 |
|------|------|
| `newer-wins` | 使用较新版本的文件 |
| `local-wins` | 始终保留本地版本 |
| `remote-wins` | 始终保留远程版本 |
| `backup` | 创建本地备份后使用远程版本 |
| `manual` | 需要手动解决冲突 |

## 示例配置

### Nextcloud 配置

```json
{
  "webdav": {
    "enabled": true,
    "url": "https://nextcloud.example.com/remote.php/dav/files/username/",
    "username": "username",
    "password": "${NEXTCLOUD_PASSWORD}",
    "remote_path": "/crush",
    "sync_interval": "10m",
    "conflict_strategy": "backup",
    "exclude_patterns": ["*.tmp", "*.lock", ".sync_state.json"]
  }
}
```

### ownCloud 配置

```json
{
  "webdav": {
    "enabled": true,
    "url": "https://owncloud.example.com/remote.php/webdav/",
    "username": "username",
    "password": "${OWNCLOUD_PASSWORD}",
    "remote_path": "/crush-config",
    "sync_interval": "5m"
  }
}
```

### Seafile 配置

```json
{
  "webdav": {
    "enabled": true,
    "url": "https://seafile.example.com/seafhttp/webdav/",
    "username": "username",
    "password": "${SEAFILE_PASSWORD}",
    "remote_path": "/crush",
    "conflict_strategy": "newer-wins"
  }
}
```

### 使用 Bearer Token

```json
{
  "webdav": {
    "enabled": true,
    "url": "https://webdav.example.com/",
    "token": "${WEBDAV_TOKEN}",
    "remote_path": "/crush"
  }
}
```

## 手动同步

可以通过命令手动触发同步：

```bash
# 同步配置（待实现）
crush sync webdav
```

## 查看同步状态

```bash
# 查看同步状态（待实现）
crush sync status
```

## 解决冲突

当检测到冲突时，可以手动解决：

```bash
# 解决冲突（待实现）
crush sync resolve <file-path> --strategy <strategy>
```

## 安全建议

1. **使用环境变量存储密码**
   ```bash
   export WEBDAV_PASSWORD="your-secret"
   ```

2. **启用 TLS**
   始终使用 HTTPS 连接 WebDAV 服务器

3. **使用专用账户**
   为 Crush 创建专用的 WebDAV 账户，限制访问权限

4. **定期备份**
   即使启用了同步，也建议定期备份配置文件

## 故障排查

详见 [故障排查指南](troubleshooting.md)

## 相关文档

- [API 文档](api.md)
- [故障排查](troubleshooting.md)
- [示例代码](../../examples/webdav-sync/)
