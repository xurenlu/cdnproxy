# CDN 代理性能优化指南

## 问题诊断

### 核心问题：地理位置导致的性能瓶颈

如果你发现通过代理下载文件（如 10MB）需要 10-15 分钟，而直接下载只需 3 秒，**根本原因通常是服务器部署位置**。

#### 问题分析

```
直接下载（3秒）：
中国用户 ──→ CDN 亚洲节点 ──→ 中国用户

通过代理下载（10-15分钟）：
中国用户 ──→ 美国代理服务器 ──→ CDN 美国节点 ──→ 美国代理 ──→ 中国用户
              ↑                                           ↓
              └───────── 数据跨太平洋两次！ ─────────────────┘
```

### 关键因素

1. **CDN 的智能路由**
   - CDN（如 jsdelivr、unpkg）会根据**客户端 IP** 选择最近的节点
   - 你的代理在美国 → CDN 给它美国节点
   - 最终用户在中国 → 数据需要跨境两次传输
   - **跨太平洋延迟**：150-300ms RTT，丢包率高

2. **User-Agent 识别**
   - 原代码使用 `User-Agent: cdnproxy/1.0`
   - CDN 无法识别为浏览器，可能：
     - 限速
     - 分配到更慢的节点
     - 当作爬虫处理

3. **TCP 窗口和缓冲区**
   - 跨境传输需要更大的 TCP 窗口
   - 默认缓冲区大小不适合高延迟网络

## 解决方案

### 🎯 方案一：优化部署位置（最重要）

**推荐部署平台（按优先级）：**

1. **Cloudflare Workers**
   - 全球 300+ 节点，自动选择最近的执行
   - 支持 KV 存储做缓存
   - 免费额度充足

2. **Vercel**
   - 有香港节点，对亚太用户友好
   - 自动全球 CDN
   - 部署简单

3. **AWS/GCP 亚洲区域**
   - Tokyo (ap-northeast-1)
   - Singapore (ap-southeast-1)
   - Hong Kong (ap-east-1)

4. **国内云服务**
   - 阿里云
   - 腾讯云
   - 华为云
   - **注意**：需要 ICP 备案

5. **自建 VPS**
   - 租用亚洲地区的 VPS
   - Vultr Tokyo/Seoul
   - Linode Tokyo/Singapore
   - DigitalOcean Singapore

### 🔧 方案二：代码优化（已实现）

#### 1. 传递真实 User-Agent

```go
// 优先使用客户端的 User-Agent
clientUA := r.Header.Get("User-Agent")
if clientUA != "" {
    req.Header.Set("User-Agent", clientUA)
} else {
    // 模拟主流浏览器
    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36...")
}
```

**效果**：让 CDN 能正确识别客户端，避免限速

#### 2. 大文件流式传输

```go
// 5MB 以上的文件不缓存，直接流式传输
if contentLength > 5 * 1024 * 1024 {
    io.Copy(w, resp.Body)  // 边下载边输出
}
```

**效果**：避免等待完整下载，立即开始传输

#### 3. HTTP Range 支持

```go
// 转发 Range 请求
if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
    req.Header.Set("Range", rangeHeader)
}
```

**效果**：支持断点续传和分段下载

#### 4. TCP 参数优化

```go
WriteBufferSize: 64 * 1024,  // 64KB 写缓冲
ReadBufferSize:  64 * 1024,  // 64KB 读缓冲
MaxConnsPerHost: 0,          // 不限制连接数
```

**效果**：适配跨境高延迟网络

### 🚀 方案三：使用国内 CDN 镜像

如果你的用户主要在中国，可以考虑：

1. **Staticfile CDN**（七牛云）
   ```
   https://cdn.staticfile.org/...
   ```

2. **BootCDN**
   ```
   https://cdn.bootcdn.net/...
   ```

3. **字节跳动 CDN**
   ```
   https://lf3-cdn-tos.bytecdntp.com/...
   ```

4. **替换策略**：
   - 检测用户位置
   - 如果在中国，重写 URL 到国内 CDN
   - 否则使用原始 CDN

### 📊 性能对比

| 场景 | 延迟 | 带宽 | 适用 |
|------|------|------|------|
| 美国代理 → 中国用户 | 300ms RTT | 1-10 Mbps | ❌ 不推荐 |
| 亚洲代理 → 中国用户 | 30-50ms | 50-100 Mbps | ✅ 推荐 |
| 国内代理 → 中国用户 | 5-20ms | 100+ Mbps | 🌟 最佳 |
| Cloudflare Workers | 自动选择最近节点 | 100+ Mbps | 🌟 推荐 |

## 高级优化技术

### 1. 多区域部署

```yaml
# 在多个地区部署代理服务器
regions:
  - us-east-1      # 服务北美用户
  - eu-west-1      # 服务欧洲用户
  - ap-southeast-1 # 服务亚太用户

# 使用 GeoDNS 自动路由到最近的服务器
```

### 2. HTTP/3 (QUIC)

- 基于 UDP，减少握手延迟
- 更好的丢包恢复
- Go 1.21+ 支持实验性 HTTP/3

```go
import "golang.org/x/net/http2"

// 启用 HTTP/3
server := &http3.Server{
    Handler: mux,
    Addr:    ":443",
}
```

### 3. BBR 拥塞控制

在 Linux 服务器上启用 BBR：

```bash
# 检查内核版本（需要 4.9+）
uname -r

# 启用 BBR
echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
sysctl -p

# 验证
sysctl net.ipv4.tcp_congestion_control
```

**效果**：在高延迟、有丢包的网络环境下，带宽利用率提升 2-25 倍

### 4. 预连接和连接复用

```go
// 对常见的 CDN 域名预建立连接
var commonCDNs = []string{
    "cdn.jsdelivr.net",
    "unpkg.com",
    "cdnjs.cloudflare.com",
}

// 启动时预热连接池
for _, cdn := range commonCDNs {
    go warmupConnection(cdn)
}
```

## 测试和验证

### 1. 测试下载速度

```bash
# 测试代理下载速度
time curl -o /dev/null "https://your-proxy.com/cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js"

# 测试直接下载速度
time curl -o /dev/null "https://cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js"
```

### 2. 测试 Range 请求

```bash
# 测试分段下载
curl -I -H "Range: bytes=0-1023" "https://your-proxy.com/cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js"

# 应该返回：
# HTTP/1.1 206 Partial Content
# Accept-Ranges: bytes
# Content-Range: bytes 0-1023/xxxxx
```

### 3. 压力测试

```bash
# 使用 ab (Apache Bench)
ab -n 1000 -c 10 "https://your-proxy.com/cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js"

# 使用 wrk
wrk -t4 -c100 -d30s "https://your-proxy.com/..."
```

## 监控指标

关键指标：

1. **TTFB (Time To First Byte)**：首字节时间
2. **下载速度**：MB/s
3. **成功率**：200 状态码比例
4. **缓存命中率**：从 Redis 返回的比例
5. **上游延迟**：到 CDN 的延迟

## 总结

**最重要的优化：选择正确的部署位置！**

代码层面的优化只能提升 10-50%，但部署位置的优化可以提升 10-100 倍。

如果你的用户在中国：
1. ✅ 部署到亚洲或中国
2. ✅ 使用国内 CDN 镜像
3. ✅ 启用 BBR 拥塞控制
4. ❌ 不要部署在美国/欧洲

