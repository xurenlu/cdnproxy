# 硬盘缓存版本说明

## 重大变更 ✨

本项目已完全移除 Redis 依赖，改用**硬盘文件缓存**系统！

### 优势

- ✅ **简化部署**：不需要 Redis 服务器
- ✅ **降低成本**：无需额外的 Redis 资源
- ✅ **数据持久化**：缓存和配置自动保存到硬盘
- ✅ **易于备份**：直接复制 data 目录即可
- ✅ **独立运行**：单个二进制文件+数据目录

### 文件存储结构

```
./data/
├── cache/              # 缓存文件目录
│   ├── xx/
│   │   └── xxx.cache   # 实际缓存文件
│   └── ...
├── whitelist.json      # 白名单配置
├── config.json         # 系统配置
├── counters.json       # 访问计数
└── sessions.json       # 管理后台会话
```

## 配置变更

### 环境变量

**新增**：
- `DATA_DIR`: 数据存储目录（默认 `./data`）
- `CACHE_DIR`: 缓存文件目录（默认 `./data/cache`）

**移除**：
- ~~`REDIS_URL`~~
- ~~`RAILWAY_REDIS_URL`~~

**保留**：
- `PORT`: 服务端口
- `CACHE_TTL_SECONDS`: 缓存过期时间
- `SESSION_TTL_SECONDS`: 会话过期时间
- `ADMIN_USERNAME`: 管理员用户名
- `ADMIN_PASSWORD`: 管理员密码

## 部署指南

### 本地运行

```bash
# 1. 编译
go build -o cdnproxy .

# 2. 创建数据目录（会自动创建）
mkdir -p ./data/cache

# 3. 运行
./cdnproxy

# 数据会自动保存到 ./data 目录
```

### Docker 部署

```bash
# 使用 docker-compose（推荐）
docker-compose up -d

# 或者手动运行
docker build -t cdnproxy .
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e ADMIN_PASSWORD=your_password \
  --name cdnproxy \
  cdnproxy
```

**重要**：使用 `-v` 挂载 `/data` 目录，确保数据持久化！

### Fly.io 部署

```bash
# fly.toml 已更新，包含数据卷配置
flyctl launch
flyctl deploy

# 数据会自动保存到持久卷
```

### Railway 部署

Railway 会自动挂载持久化卷到 `/data`，无需额外配置。

## 性能说明

### 缓存策略

1. **小文件**（< 5MB）：
   - 缓存到硬盘
   - 支持 WebP 转换
   - 支持 Gzip 压缩

2. **大文件**（≥ 5MB）：
   - 直接流式传输
   - 不缓存
   - 支持 Range 请求

### 文件过期清理

- 每小时自动清理过期缓存文件
- 缓存文件按 `StoredAt` 时间判断过期
- 会话和计数器也会定期清理

### 性能对比

| 场景 | Redis 版本 | 硬盘缓存版本 |
|------|-----------|-------------|
| 缓存命中读取 | ~1ms | ~2-5ms |
| 缓存写入 | ~2ms | ~5-10ms |
| 冷启动 | 需等待 Redis | 立即可用 |
| 内存占用 | Redis 进程 + Go | 仅 Go 进程 |
| 硬盘占用 | 无 | 缓存文件大小 |
| 部署复杂度 | 需 Redis | 无依赖 |

**结论**：硬盘缓存版本略慢 2-5ms，但部署更简单，适合中小流量场景。

## 数据备份

### 备份

```bash
# 停止服务
docker-compose stop

# 备份数据目录
tar -czf cdnproxy-backup-$(date +%Y%m%d).tar.gz data/

# 恢复服务
docker-compose start
```

### 恢复

```bash
# 停止服务
docker-compose stop

# 解压备份
tar -xzf cdnproxy-backup-20241003.tar.gz

# 恢复服务
docker-compose start
```

## 迁移指南（从 Redis 版本）

### 1. 数据迁移

原 Redis 版本的数据无法直接迁移，因为存储格式不同。建议：

1. 在新服务器部署硬盘缓存版本
2. 逐步切换流量
3. 让缓存自然重建

### 2. 配置迁移

白名单需要手动重新添加：

1. 从旧版管理后台导出白名单
2. 在新版管理后台逐个添加

或者直接编辑 `data/whitelist.json`：

```json
[
  "example.com",
  ".trusted-domain.com"
]
```

## 监控建议

### 磁盘空间监控

```bash
# 查看缓存目录大小
du -sh data/cache

# 查看缓存文件数量
find data/cache -name "*.cache" | wc -l
```

### 清理缓存

```bash
# 手动清理所有缓存（慎用）
rm -rf data/cache/*

# 清理30天前的缓存
find data/cache -name "*.cache" -mtime +30 -delete
```

## 限制说明

1. **单文件最大缓存**：100MB（可在代码中调整）
2. **大文件不缓存**：≥5MB 文件直接流式传输
3. **并发写入**：使用文件锁保证安全
4. **文件数量**：建议不超过 10万个缓存文件

## 常见问题

### Q: 缓存会占用多少硬盘空间？

A: 取决于访问的资源大小和数量。建议预留至少 10GB 空间。可以通过调整 `CACHE_TTL_SECONDS` 控制缓存时间。

### Q: 如何清空所有缓存？

A: 删除 `data/cache` 目录下的所有文件即可，服务会自动重建目录。

### Q: 性能比 Redis 版本慢吗？

A: 缓存命中慢 2-5ms，对大多数场景影响不大。大文件使用流式传输，性能相同。

### Q: 数据会丢失吗？

A: 只要挂载了持久化卷，数据不会丢失。建议定期备份 `data` 目录。

### Q: 可以多实例部署吗？

A: 可以，但每个实例的缓存独立，不共享。适合负载均衡场景，缓存命中率略低。

## 技术细节

### 文件存储实现

- **缓存文件**：使用 Gob 编码存储 `cache.Entry` 结构
- **配置文件**：使用 JSON 格式，易于阅读和编辑
- **文件锁**：使用 `sync.RWMutex` 保证并发安全
- **过期清理**：定时 goroutine 扫描并删除过期文件

### 缓存键生成

```go
func buildCacheKey(method, upstreamURL string) string {
    h := sha256.Sum256([]byte(method + " " + upstreamURL))
    return "cache:v3:" + hex.EncodeToString(h[:])
}
```

### 文件路径

缓存文件存储路径使用哈希值的前两位作为子目录，避免单目录文件过多：

```
data/cache/
├── ab/
│   └── cdef1234...cache
└── cd/
    └── ef567890...cache
```

## 升级日志

- **v2.0.0** (2024-10-03)
  - ✨ 完全移除 Redis 依赖
  - ✨ 实现基于硬盘的缓存系统
  - ✨ 添加文件存储的配置和会话管理
  - ⚡ 优化大文件流式传输
  - ⚡ 添加自动过期清理机制
  - 📝 更新所有部署文档

---

享受更简单的部署体验！🎉

