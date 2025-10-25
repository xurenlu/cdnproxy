# 性能优化指南

## 当前潜在问题

### 1. 内存消耗问题

**问题描述：**
- 小文件（<5MB）会全量读入内存
- 解压、转换、压缩过程中会创建多个数据副本
- 高并发时可能导致内存耗尽

**内存占用计算：**
```
原始文件 + 解压后 + WebP转换 + 压缩后 = 4倍内存占用
1MB 文件可能占用 4MB 内存
```

### 2. 硬盘 I/O 问题

**问题描述：**
- 频繁的文件读写操作
- 清理过期文件时遍历整个缓存目录
- 高并发时 I/O 竞争

## 优化建议

### 1. 内存优化

#### 1.1 流式处理小文件
```go
// 建议：即使是小文件也使用流式处理
if contentLength > 1*1024*1024 { // 1MB 阈值
    // 流式传输，不读入内存
    io.Copy(w, resp.Body)
    return
}
```

#### 1.2 限制并发处理
```go
// 建议：添加并发限制
var semaphore = make(chan struct{}, 10) // 最多10个并发处理

func (h *Handler) processWithLimit() {
    semaphore <- struct{}{}
    defer func() { <-semaphore }()
    // 处理逻辑
}
```

#### 1.3 内存池复用
```go
// 建议：使用 sync.Pool 复用缓冲区
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 64*1024) // 64KB 缓冲区
    },
}
```

### 2. 硬盘 I/O 优化

#### 2.1 异步清理
```go
// 建议：异步清理，避免阻塞主流程
go func() {
    time.Sleep(5 * time.Minute) // 延迟清理
    h.cache.Cleanup(context.Background())
}()
```

#### 2.2 批量清理
```go
// 建议：批量删除文件，减少系统调用
func (d *DiskCache) BatchCleanup(ctx context.Context) error {
    var filesToDelete []string
    
    filepath.Walk(d.basePath, func(path string, info os.FileInfo, err error) error {
        if time.Since(info.ModTime()) > 24*time.Hour {
            filesToDelete = append(filesToDelete, path)
        }
        return nil
    })
    
    // 批量删除
    for _, file := range filesToDelete {
        os.Remove(file)
    }
}
```

### 3. 配置优化

#### 3.1 环境变量配置
```bash
# 建议的配置
export MAX_CACHE_FILE_SIZE=104857600    # 100MB
export MAX_CONCURRENT_REQUESTS=50       # 最大并发数
export CACHE_CLEANUP_INTERVAL=3600      # 清理间隔（秒）
export MEMORY_LIMIT_MB=512              # 内存限制
```

#### 3.2 监控指标
```go
// 建议：添加监控指标
type Metrics struct {
    CacheHits     int64
    CacheMisses   int64
    MemoryUsage   int64
    DiskUsage     int64
    ActiveConns   int64
}
```

### 4. 部署优化

#### 4.1 系统级限制
```bash
# 限制进程内存使用
ulimit -v 1048576  # 1GB 虚拟内存限制

# 限制文件描述符
ulimit -n 4096
```

#### 4.2 Docker 资源限制
```yaml
services:
  cdnproxy:
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '1.0'
        reservations:
          memory: 256M
          cpus: '0.5'
```

### 5. 代码优化建议

#### 5.1 减少内存分配
```go
// 当前代码问题
body, err := io.ReadAll(resp.Body) // 全量读取

// 建议优化
var buf bytes.Buffer
buf.Grow(int(contentLength)) // 预分配容量
io.Copy(&buf, resp.Body)
body := buf.Bytes()
```

#### 5.2 流式 WebP 转换
```go
// 建议：流式处理图片转换
func (h *Handler) convertToWebPStream(src io.Reader, dst io.Writer) error {
    // 使用流式解码和编码
    // 避免全量读入内存
}
```

## 监控建议

### 1. 关键指标监控
- **内存使用率**：`runtime.MemStats`
- **Goroutine 数量**：`runtime.NumGoroutine()`
- **文件描述符使用**：`/proc/self/fd`
- **硬盘 I/O**：`iostat` 或 `/proc/diskstats`

### 2. 告警阈值
- 内存使用率 > 80%
- Goroutine 数量 > 1000
- 文件描述符 > 80% 限制
- 硬盘 I/O 等待时间 > 100ms

### 3. 日志监控
```go
// 建议：添加性能日志
log.Printf("memory: %dMB, goroutines: %d, cache_size: %dMB", 
    memStats.Alloc/1024/1024,
    runtime.NumGoroutine(),
    cacheSize/1024/1024)
```

## 应急处理

### 1. 内存泄漏处理
```bash
# 重启服务
systemctl restart cdnproxy

# 清理缓存
rm -rf /data/cache/*

# 监控内存
watch -n 1 'ps aux | grep cdnproxy'
```

### 2. 硬盘空间不足
```bash
# 清理过期缓存
find /data/cache -name "*.cache" -mtime +1 -delete

# 压缩日志
gzip /var/log/cdnproxy/*.log
```

## 测试建议

### 1. 压力测试
```bash
# 使用 ab 进行压力测试
ab -n 1000 -c 50 http://localhost:8080/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js

# 使用 wrk 进行长时间测试
wrk -t12 -c400 -d30s http://localhost:8080/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
```

### 2. 内存泄漏测试
```bash
# 使用 pprof 分析内存
go tool pprof http://localhost:8080/debug/pprof/heap
```

## 总结

当前项目在高并发和大文件场景下确实存在内存和 I/O 风险。建议：

1. **立即优化**：降低小文件处理阈值，增加并发限制
2. **中期优化**：实现流式处理，减少内存分配
3. **长期优化**：添加监控和自动恢复机制

通过以上优化，可以显著降低资源消耗风险。