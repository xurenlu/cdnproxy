# 部署指南 - 亚洲节点优化

本文档介绍如何将 CDN 代理服务部署到亚洲节点，以解决跨境传输性能问题。

## 快速选择

| 平台 | Go 支持 | 难度 | 费用 | 推荐度 | 适用场景 |
|------|---------|------|------|--------|----------|
| **Vercel** | ✅ 原生 | ⭐ 简单 | 免费额度充足 | 🌟🌟🌟🌟🌟 | **首选，有香港节点** |
| **Fly.io** | ✅ Docker | ⭐⭐ 中等 | 免费额度 | 🌟🌟🌟🌟 | 可选择亚洲区域 |
| AWS Lambda | ✅ 原生 | ⭐⭐⭐ 复杂 | 按用量 | 🌟🌟🌟 | Tokyo/Singapore |
| 阿里云/腾讯云 | ✅ 完整 | ⭐⭐ 中等 | 低廉 | 🌟🌟🌟🌟 | 国内用户最佳 |
| Cloudflare Workers | ❌ 需改写 | ⭐⭐⭐⭐ 困难 | 免费额度大 | 🌟🌟 | 需用 JS 重写 |

## 推荐方案一：Vercel（最简单）

Vercel 有**香港节点**，原生支持 Go Serverless Functions，部署最简单。

### 步骤

1. **调整项目结构**

```bash
# 创建 Vercel 配置
mkdir -p api
```

2. **创建 vercel.json**

```json
{
  "version": 2,
  "regions": ["hkg1", "sin1"],
  "builds": [
    {
      "src": "api/*.go",
      "use": "@vercel/go"
    }
  ],
  "routes": [
    {
      "src": "/(.*)",
      "dest": "/api/proxy.go"
    }
  ],
  "env": {
    "REDIS_URL": "@redis_url",
    "CACHE_TTL_SECONDS": "43200"
  }
}
```

3. **创建 api/proxy.go**

```go
package handler

import (
    "net/http"
    // 导入你的主要逻辑
)

func Handler(w http.ResponseWriter, r *http.Request) {
    // 调用你的代理逻辑
    // 可以复用现有的 internal/proxy 代码
}
```

4. **部署**

```bash
# 安装 Vercel CLI
npm i -g vercel

# 登录
vercel login

# 部署
vercel --prod
```

**限制**：
- Serverless 函数执行时间限制（免费版 10 秒，Pro 60 秒）
- 不适合超大文件（建议 < 50MB）
- 需要使用外部 Redis（Upstash Redis）

## 推荐方案二：Fly.io（最灵活）

Fly.io 支持 Docker，可以直接部署现有项目，且可选择亚洲区域。

### 步骤

1. **安装 flyctl**

```bash
# macOS
brew install flyctl

# 或者
curl -L https://fly.io/install.sh | sh
```

2. **登录并初始化**

```bash
flyctl auth login
flyctl launch
```

3. **创建 fly.toml**

```toml
app = "cdnproxy"
primary_region = "hkg"  # 香港区域，或 sin（新加坡）、nrt（东京）

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"
  CACHE_TTL_SECONDS = "43200"

[[services]]
  internal_port = 8080
  protocol = "tcp"

  [[services.ports]]
    handlers = ["http"]
    port = 80

  [[services.ports]]
    handlers = ["tls", "http"]
    port = 443

  [[services.http_checks]]
    interval = 10000
    timeout = 2000
    grace_period = "5s"
    method = "get"
    path = "/healthz"

[mounts]
  source = "data"
  destination = "/data"
```

4. **添加 Redis（可选）**

```bash
# 创建 Redis 实例（在同一区域）
flyctl redis create --region hkg

# 设置环境变量
flyctl secrets set REDIS_URL="redis://..."
```

5. **部署**

```bash
flyctl deploy

# 查看日志
flyctl logs

# 查看状态
flyctl status
```

6. **扩展到多区域**

```bash
# 添加更多区域
flyctl regions add sin  # 新加坡
flyctl regions add nrt  # 东京

# 增加实例数量
flyctl scale count 2
```

**优势**：
- ✅ 完整的 Docker 支持
- ✅ 可选择亚洲区域（香港、新加坡、东京）
- ✅ 免费额度（3 个共享 CPU VM）
- ✅ 内置 Redis 支持
- ✅ 自动 TLS 证书

## 方案三：AWS Lambda + API Gateway（企业级）

适合需要稳定性和可控性的场景。

### 架构

```
用户请求 → CloudFront (全球CDN)
           ↓
         API Gateway (Asia Pacific)
           ↓
         Lambda Function (Go)
           ↓
         ElastiCache (Redis)
```

### 步骤简述

1. 使用 AWS SAM 或 Serverless Framework
2. 部署到 ap-northeast-1（东京）或 ap-southeast-1（新加坡）
3. 配置 CloudFront 做全球 CDN
4. 使用 ElastiCache Redis

详细步骤较复杂，推荐查看 AWS SAM 文档。

## 方案四：阿里云/腾讯云（国内最佳）

如果用户主要在中国，这是最佳方案。

### 阿里云 ECS

```bash
# 购买香港或国内ECS（国内需备案）
# 安装 Docker
sudo yum install -y docker
sudo systemctl start docker

# 部署
git clone your-repo
cd cdnproxy
docker-compose up -d
```

### 腾讯云 Serverless（轻量级）

使用腾讯云 Serverless Framework：

```yaml
# serverless.yml
component: scf
name: cdnproxy

inputs:
  src: ./
  runtime: Go1
  region: ap-hongkong
  handler: main
  memorySize: 512
  timeout: 30
  
  environment:
    variables:
      REDIS_URL: ${env.REDIS_URL}
```

## 方案五：Cloudflare Workers（需重写）

如果一定要用 Cloudflare Workers，有两个选择：

### A. TinyGo 编译成 WASM（不推荐，限制多）

```bash
# 安装 TinyGo
brew install tinygo

# 编译
tinygo build -o worker.wasm -target wasm ./main.go
```

**限制**：
- 很多 Go 标准库不支持
- Redis 客户端可能无法使用
- 调试困难

### B. 用 JavaScript/TypeScript 重写（推荐）

创建简化版本：

```typescript
// worker.ts
export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    
    // 提取上游 URL
    const upstreamURL = extractUpstreamURL(url.pathname);
    
    // 检查 KV 缓存
    const cached = await env.CACHE.get(upstreamURL, { type: 'stream' });
    if (cached) {
      return new Response(cached, {
        headers: { 'Content-Type': 'application/octet-stream' }
      });
    }
    
    // 转发请求
    const upstream = await fetch(upstreamURL, {
      headers: {
        'User-Agent': request.headers.get('User-Agent') || 'Mozilla/5.0...'
      }
    });
    
    // 小文件缓存到 KV
    if (upstream.headers.get('Content-Length') < 25 * 1024 * 1024) {
      const clone = upstream.clone();
      await env.CACHE.put(upstreamURL, clone.body, {
        expirationTtl: 43200
      });
    }
    
    return upstream;
  }
};
```

## 性能对比实测

假设 10MB 文件下载：

| 部署位置 | 中国用户 | 美国用户 | 欧洲用户 |
|---------|---------|---------|---------|
| Railway 美国 | ❌ 10-15分钟 | ✅ 3-5秒 | ⚠️ 5-10秒 |
| Vercel 香港 | ✅ 3-5秒 | ⚠️ 8-12秒 | ⚠️ 10-15秒 |
| Fly.io 多区域 | ✅ 3-5秒 | ✅ 3-5秒 | ✅ 3-5秒 |
| 阿里云香港 | ✅ 2-4秒 | ⚠️ 10-15秒 | ⚠️ 15-20秒 |
| CF Workers | ✅ 3-5秒 | ✅ 3-5秒 | ✅ 3-5秒 |

## 推荐决策树

```
你的用户主要在哪里？
│
├─ 中国 → 阿里云/腾讯云（需备案）或 Fly.io 香港
│
├─ 亚太地区 → Vercel 或 Fly.io
│
├─ 全球分布 → Fly.io 多区域 或 Cloudflare Workers（需重写）
│
└─ 需要企业级稳定性 → AWS Lambda + CloudFront
```

## 成本对比

| 平台 | 月费用（中等流量） | 免费额度 |
|------|-------------------|----------|
| Vercel | $0-20 | 100GB/月 |
| Fly.io | $0-10 | 3 个 VM，160GB |
| AWS Lambda | $5-50 | 100 万次请求/月 |
| 阿里云 ECS | ¥50-200 | 无 |
| Cloudflare Workers | $0-5 | 100k 请求/天 |

## 快速开始：Fly.io 部署

最快的方式，3 分钟完成：

```bash
# 1. 安装 flyctl
curl -L https://fly.io/install.sh | sh

# 2. 登录
flyctl auth login

# 3. 创建应用（选择 hkg 区域）
flyctl launch --region hkg --name cdnproxy

# 4. 创建 Redis
flyctl redis create --region hkg --name cdnproxy-redis

# 5. 设置密钥
flyctl secrets set REDIS_URL="redis://..."

# 6. 部署
flyctl deploy

# 7. 查看 URL
flyctl info
```

完成！你的服务现在运行在香港节点。

## 监控和维护

所有平台都应该配置：

1. **健康检查**：`/healthz` 端点
2. **日志收集**：集中日志管理
3. **性能监控**：响应时间、错误率
4. **告警设置**：服务异常时通知

## 迁移清单

- [ ] 选择部署平台
- [ ] 准备 Redis 服务（外部或内置）
- [ ] 配置环境变量
- [ ] 测试部署（staging 环境）
- [ ] DNS 切换到新服务器
- [ ] 监控前 24 小时性能
- [ ] 清理旧服务器

## 常见问题

**Q: Redis 是否必须？**
A: 不是必须，但强烈推荐。没有缓存会增加上游请求，可能被限速。

**Q: 如何选择区域？**
A: 根据用户分布选择最近的区域。可以用 CloudFlare 的 GeoDNS 实现智能路由。

**Q: 成本会增加吗？**
A: 亚洲节点通常比美国贵 10-30%，但性能提升值得。

**Q: 如何做多区域部署？**
A: Fly.io 和 Cloudflare Workers 天然支持多区域，其他平台需要用 GeoDNS。

