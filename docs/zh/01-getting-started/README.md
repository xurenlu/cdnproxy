# 快速开始

## 📖 什么是 CDNProxy？

CDNProxy 是一个高性能的 CDN 和 AI API 代理服务，用于在受限网络环境下访问公共资源。

**核心特性**:
- ✅ CDN 资源代理（支持缓存）
- ✅ AI API 代理（OpenAI、Claude、Gemini 等）
- ✅ WebSocket 和 SSE 支持
- ✅ 硬盘缓存（无需 Redis）
- ✅ 访问控制和白名单管理

## 🚀 安装运行

### 方式一：直接运行

```bash
# 编译
go build -o cdnproxy .

# 运行（默认端口 8080）
./cdnproxy
```

### 方式二：Docker

```bash
# 使用 docker-compose
docker-compose up -d

# 或手动运行
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e ADMIN_PASSWORD=your_password \
  cdnproxy
```

### 方式三：源码编译

```bash
git clone https://github.com/xurenlu/cdnproxy.git
cd cdnproxy
go build -o cdnproxy .
./cdnproxy
```

## ⚙️ 基础配置

### 环境变量

```bash
# 端口（默认 8080）
export PORT=8080

# 数据目录（默认 ./data）
export DATA_DIR=./data

# 管理员密码（必须修改！）
export ADMIN_PASSWORD=your_secure_password

# 缓存 TTL（默认 12 小时）
export CACHE_TTL_SECONDS=43200
```

### 管理界面

访问 `http://localhost:8080/admin/` 进行白名单管理

默认账号：`admin` / `cdnproxy123!`（生产环境请修改）

## 🎯 使用示例

### CDN 资源代理

**规则**: 将原始 URL 的 `https://` 替换为 `https://your-proxy-domain/`

```bash
# 原始 URL
https://cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js

# 通过代理访问
https://your-proxy-domain/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
```

### AI API 代理

**规则**: 同样替换域名，保持所有请求头和参数

```bash
# OpenAI API
curl https://your-proxy-domain/api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'

# Claude API
curl https://your-proxy-domain/api.anthropic.com/v1/messages \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 🔒 访问控制

CDN 代理请求需要满足以下任一条件：

1. **非常见浏览器 User-Agent**（如 curl、wget）
2. **Referer 为 IP 或本地开发**（localhost、127.0.0.1）
3. **域名在白名单中**（在 `/admin/` 管理台添加）

**注意**: API 代理请求不受访问控制限制。

## 📚 下一步

- 📖 [API 代理使用指南](../02-user-guide/api-proxy.md)
- 🏠 [住宅 IP 代理配置](../02-user-guide/residential-proxy.md)
- 🚢 [部署指南](../04-deployment/docker.md)
- ⚡ [性能优化](../05-advanced/performance.md)

## 💡 常见问题

**Q: 如何添加自定义 API 域名？**  
A: 设置环境变量 `API_DOMAINS=api.example.com,api2.example.com`

**Q: 缓存如何清理？**  
A: 删除 `data/cache` 目录或通过管理界面操作

**Q: 支持 WebSocket 吗？**  
A: 支持，API 代理自动识别 WebSocket 升级请求

**Q: 如何查看日志？**  
A: 日志输出到标准输出，可通过 Docker logs 或重定向查看
