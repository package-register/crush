# WebDAV 同步故障排查指南

## 常见问题

### 1. 连接失败

**错误信息**: `webdav: connection failed`

**可能原因**:
- WebDAV 服务器 URL 不正确
- 网络连接问题
- 服务器不可用
- TLS/SSL 证书问题

**解决方案**:

```bash
# 1. 检查 URL 是否正确
curl -X OPTIONS https://webdav.example.com/

# 2. 检查网络连接
ping webdav.example.com

# 3. 测试 TLS 证书
curl -v https://webdav.example.com/

# 4. 临时跳过 TLS 验证（仅测试）
# 在配置中设置 skip_tls_verify: true
```

### 2. 认证失败

**错误信息**: `webdav: unauthorized (status: 401)`

**可能原因**:
- 用户名或密码错误
- Token 已过期
- 认证方式不正确

**解决方案**:

```bash
# 1. 验证凭证
curl -u username:password -X PROPFIND https://webdav.example.com/

# 2. 检查环境变量
echo $WEBDAV_PASSWORD

# 3. 确认认证方式
# 某些服务器使用 Bearer Token 而非基本认证
```

### 3. 权限不足

**错误信息**: `webdav: forbidden (status: 403)`

**可能原因**:
- 用户对目标目录没有写权限
- 目录不存在

**解决方案**:

```bash
# 1. 检查目录权限
curl -u username:password -X PROPFIND https://webdav.example.com/crush/

# 2. 创建目录
curl -u username:password -X MKCOL https://webdav.example.com/crush/

# 3. 检查用户权限
# 在 WebDAV 服务器管理界面确认用户权限
```

### 4. 同步冲突

**错误信息**: `webdav: sync conflict detected`

**可能原因**:
- 同一文件在本地和远程都被修改
- 同步间隔太短

**解决方案**:

```bash
# 1. 查看冲突文件
# 检查日志中的冲突信息

# 2. 手动解决冲突
# 使用配置中的 conflict_strategy 选项

# 3. 调整同步间隔
# 增加 sync_interval 时间

# 4. 查看备份文件
# 如果使用 backup 策略，检查 .backup.* 文件
```

### 5. 文件不同步

**问题**: 文件没有自动同步

**可能原因**:
- 同步未启动
- 同步间隔未设置
- 文件被排除

**解决方案**:

```bash
# 1. 检查同步状态
# 查看日志确认同步是否运行

# 2. 设置同步间隔
# 在配置中添加 "sync_interval": "5m"

# 3. 检查排除模式
# 确认文件没有被 exclude_patterns 排除

# 4. 手动触发同步
# 调用 Sync() 方法
```

### 6. 大文件同步失败

**错误信息**: `webdav: request timeout`

**可能原因**:
- 文件太大，上传超时
- 网络速度慢

**解决方案**:

```bash
# 1. 增加超时时间
# 在代码中设置 WithTimeout(5 * time.Minute)

# 2. 检查网络速度

# 3. 分块上传
# （未来功能）
```

## 日志分析

### 启用调试日志

```json
{
  "options": {
    "debug": true
  },
  "webdav": {
    "enabled": true
  }
}
```

### 日志位置

日志默认输出到标准错误，可以通过重定向保存：

```bash
crush 2> crush.log
```

### 关键日志信息

```
[INFO] Starting WebDAV sync
[INFO] Connected to WebDAV server
[DEBUG] File uploaded path=config.json size=1024
[DEBUG] File downloaded path=settings.json size=2048
[WARN] Sync conflict detected path=config.json
[ERROR] Sync error path=config.json error=...
```

## 性能优化

### 1. 调整同步间隔

```json
{
  "webdav": {
    "sync_interval": "10m"
  }
}
```

### 2. 排除不必要的文件

```json
{
  "webdav": {
    "exclude_patterns": ["*.tmp", "*.log", "*.backup.*"]
  }
}
```

### 3. 使用增量同步

系统默认使用 ETag 进行增量同步，确保服务器支持 ETag。

## 服务器特定问题

### Nextcloud

**问题**: 路径不正确

**解决方案**: Nextcloud WebDAV URL 格式：
```
https://nextcloud.example.com/remote.php/dav/files/username/
```

### ownCloud

**问题**: 认证失败

**解决方案**: ownCloud 可能需要应用专用密码：
```
在 ownCloud 设置中生成应用密码
```

### Seafile

**问题**: 路径限制

**解决方案**: Seafile WebDAV 路径：
```
https://seafile.example.com/seafhttp/webdav/
```

## 获取帮助

### 收集诊断信息

```bash
# 1. 收集配置信息（移除敏感信息）
cat crush.json | jq '.webdav'

# 2. 收集日志
tail -n 100 crush.log

# 3. 测试连接
curl -v -u username:password -X PROPFIND https://webdav.example.com/
```

### 报告问题

报告问题时请提供：
1. Crush 版本
2. WebDAV 服务器类型和版本
3. 配置信息（移除敏感信息）
4. 相关日志
5. 复现步骤

## 相关资源

- [配置指南](configuration.md)
- [API 文档](api.md)
- [示例代码](../../examples/webdav-sync/)
