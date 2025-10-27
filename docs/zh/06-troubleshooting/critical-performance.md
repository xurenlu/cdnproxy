# 关键性能问题修复指南

## 🚨 严重问题列表

### 1. 连接池无限制 (CRITICAL)

**问题：**
```go
MaxConnsPerHost: 0,  // 无限制 - 危险！
```

**风险：**
- 文件描述符耗尽
- 内存耗尽
- 服务器崩溃

**修复：**
```go
tr := &http.Transport{
    MaxIdleConns:        100,  // 降低到合理值
    MaxIdleConnsPerHost: 50,   // 降低到合理值
    MaxConnsPerHost:     100,  // 添加限制！
    IdleConnTimeout:     30 * time.Second,  // 缩短空闲时间
}
```

### 2. 无超时控制 (CRITICAL)

**问题：**
```go
Timeout: 0,  // 无超时 - 危险！
ReadTimeout: 0,  // 无限制
WriteTimeout: 0,  // 无限制
```

**风险：**
- 僵尸连接
- 资源耗尽
- DoS 攻击

**修复：**
```go
// HTTP 客户端超时
httpClient: &http.Client{
    Transport: tr,
    Timeout:   30 * time.Second,  // 添加超时！
}

// 服务器超时
srv := &http.Server{
    ReadHeaderTimeout: 10 * time.Second,
    ReadTimeout:       30 * time.Second,  // 添加读取超时
    WriteTimeout:      30 * time.Second,  // 添加写入超时
    IdleTimeout:       60 * time.Second,  // 缩短空闲超时
}
```

### 3. Goroutine 泄漏 (HIGH)

**问题：**
- WebSocket 处理无超时
- SSE 流式处理无限制
- 缓存清理可能重叠

**修复：**
```go
// WebSocket 超时控制
func (h *Handler) proxyWebSocket(w http.ResponseWriter, r *http.Request, upstreamURL string) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
    defer cancel()
    
    // 使用带超时的 context
    upstreamConn, err := h.dialUpstreamWithTimeout(ctx, wsURL, upstreamReq)
}

// SSE 流式处理超时
func (h *Handler) streamSSE(w http.ResponseWriter, body io.Reader) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()
    
    // 使用带超时的 context
    scanner := bufio.NewScanner(body)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return
        default:
            // 处理数据
        }
    }
}
```

### 4. 文件描述符限制 (HIGH)

**问题：**
- 无 fd 限制
- 可能耗尽系统资源

**修复：**
```go
// 在 main.go 中添加
func main() {
    // 设置文件描述符限制
    if err := setFDLimit(4096); err != nil {
        log.Printf("Failed to set FD limit: %v", err)
    }
}

func setFDLimit(limit int) error {
    var rLimit syscall.Rlimit
    rLimit.Cur = uint64(limit)
    rLimit.Max = uint64(limit)
    return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}
```

### 5. 内存池优化 (MEDIUM)

**问题：**
- 频繁的 `[]byte` 分配
- 内存碎片化

**修复：**
```go
// 添加内存池
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 64*1024) // 64KB 缓冲区
    },
}

// 使用内存池
func (h *Handler) processWithPool() {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    
    // 使用 buf 处理数据
}
```

## 🔧 立即修复代码

### 1. 修复连接池配置

```go
// internal/proxy/handler.go
func NewHandler(cfg config.Config, diskCache *cache.DiskCache, whitelistStore WhitelistStore, configStore ConfigStore, counterStore CounterStore) http.Handler {
    dialer := &net.Dialer{
        Timeout:   10 * time.Second,  // 缩短连接超时
        KeepAlive: 30 * time.Second,  // 缩短保活时间
    }
    
    tr := &http.Transport{
        Proxy:                 http.ProxyFromEnvironment,
        DialContext:           dialer.DialContext,
        ForceAttemptHTTP2:     true,
        MaxIdleConns:          50,   // 降低
        MaxIdleConnsPerHost:   25,   // 降低
        MaxConnsPerHost:       100,  // 添加限制！
        IdleConnTimeout:       30 * time.Second,  // 缩短
        TLSHandshakeTimeout:   5 * time.Second,   // 缩短
        ExpectContinueTimeout: 1 * time.Second,
        ResponseHeaderTimeout: 10 * time.Second,  // 缩短
        DisableCompression:    true,
        WriteBufferSize:       32 * 1024,  // 降低
        ReadBufferSize:        32 * 1024,  // 降低
    }
    
    return &Handler{
        cfg:            cfg,
        cache:          diskCache,
        whitelistStore: whitelistStore,
        configStore:    configStore,
        counterStore:   counterStore,
        httpClient:     &http.Client{
            Transport: tr,
            Timeout:   30 * time.Second,  // 添加超时！
        },
    }
}
```

### 2. 修复服务器超时

```go
// main.go
srv := &http.Server{
    Addr:              ":" + cfg.Port,
    Handler:           loggingMiddleware(mux),
    ReadHeaderTimeout: 10 * time.Second,
    ReadTimeout:       30 * time.Second,  // 添加读取超时
    WriteTimeout:      30 * time.Second,  // 添加写入超时
    IdleTimeout:       60 * time.Second,  // 缩短空闲超时
}
```

### 3. 添加并发限制

```go
// internal/proxy/handler.go
type Handler struct {
    cfg            config.Config
    cache          *cache.DiskCache
    whitelistStore WhitelistStore
    httpClient     *http.Client
    configStore    ConfigStore
    counterStore   CounterStore
    semaphore      chan struct{}  // 添加信号量
}

func NewHandler(cfg config.Config, diskCache *cache.DiskCache, whitelistStore WhitelistStore, configStore ConfigStore, counterStore CounterStore) http.Handler {
    // ... 现有代码 ...
    
    return &Handler{
        cfg:            cfg,
        cache:          diskCache,
        whitelistStore: whitelistStore,
        configStore:    configStore,
        counterStore:   counterStore,
        httpClient:     &http.Client{Transport: tr, Timeout: 30 * time.Second},
        semaphore:      make(chan struct{}, 50),  // 最多50个并发
    }
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 获取信号量
    select {
    case h.semaphore <- struct{}{}:
        defer func() { <-h.semaphore }()
    case <-r.Context().Done():
        http.Error(w, "service busy", http.StatusServiceUnavailable)
        return
    }
    
    // ... 现有处理逻辑 ...
}
```

### 4. 添加监控

```go
// 添加性能监控
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        if duration > 5*time.Second {
            log.Printf("SLOW REQUEST: %s %s %s", r.Method, r.URL.Path, duration)
        }
    }()
    
    // ... 现有处理逻辑 ...
}
```

## 🚨 紧急部署建议

### 1. 立即修复
```bash
# 1. 设置系统限制
ulimit -n 4096
ulimit -v 1048576  # 1GB 虚拟内存

# 2. 重启服务
systemctl restart cdnproxy

# 3. 监控资源使用
watch -n 1 'ps aux | grep cdnproxy'
```

### 2. 监控指标
```bash
# 监控文件描述符
lsof -p $(pgrep cdnproxy) | wc -l

# 监控内存使用
ps -o pid,vsz,rss,comm -p $(pgrep cdnproxy)

# 监控连接数
netstat -an | grep :8080 | wc -l
```

### 3. 告警阈值
- 文件描述符 > 3000
- 内存使用 > 500MB
- 连接数 > 200
- 响应时间 > 5秒

## 📊 性能测试

### 1. 压力测试
```bash
# 测试连接限制
ab -n 1000 -c 100 http://localhost:8080/healthz

# 测试大文件
curl -o /dev/null -s -w "%{time_total}\n" http://localhost:8080/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
```

### 2. 内存泄漏测试
```bash
# 使用 pprof 分析
go tool pprof http://localhost:8080/debug/pprof/heap
```

## 总结

这些修复应该立即实施，特别是：
1. **连接池限制** - 防止资源耗尽
2. **超时控制** - 防止僵尸连接
3. **并发限制** - 防止过载
4. **监控告警** - 及时发现问题

修复后，系统稳定性将显著提升。
