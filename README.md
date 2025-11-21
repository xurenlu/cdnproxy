# CDNProxy v2.2

一个用于在受限网络环境下代理访问公共 CDN 资源和 AI API 服务的 Go 代理服务。

## 🎯 核心功能

### CDN 代理
- **路径代理**: `/host/path` → `https://host/path`
- **智能缓存**: 硬盘缓存，默认 12 小时 TTL
- **访问控制**: 基于 User-Agent、Referer 和白名单
- **流式传输**: 支持大文件和 Range 请求

### API 代理
- **AI 服务**: 支持 OpenAI、Claude、Gemini、Poe 等
- **实时通信**: 支持 WebSocket 和 SSE 流式响应
- **无限制**: 无大小限制、无超时限制
- **自动识别**: 自动识别 API 域名，无需配置

## 🚀 快速开始

### 安装运行

```bash
# 编译
go build -o cdnproxy .

# 运行（默认端口 8080）
./cdnproxy
```

### Docker 部署

```bash
docker-compose up -d
# 或
docker run -d -p 8080:8080 -v $(pwd)/data:/data -e ADMIN_PASSWORD=your_password cdnproxy
```

## 📖 使用方法

### CDN 资源代理

**规则**: 将原始 URL 的 `https://` 替换为 `https://your-proxy-domain/`

```bash
# 原始: https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
# 代理: https://your-proxy-domain/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

### API 服务代理

**规则**: 同样将 `https://` 替换为代理域名

```bash
# OpenAI API
curl https://your-proxy-domain/api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_KEY" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello!"}]}'

# Claude API
curl https://your-proxy-domain/api.anthropic.com/v1/messages \
  -H "Authorization: Bearer YOUR_KEY" \
  -d '{"model":"claude-3-opus","messages":[{"role":"user","content":"Hello!"}]}'
```

**支持的 API 域名**:
- `api.openai.com` - OpenAI
- `api.anthropic.com` - Claude
- `poe.com` / `api.poe.com` - Poe
- `generativelanguage.googleapis.com` - Google Gemini
- `api.cohere.ai` - Cohere
- `api.together.xyz` - Together AI
- `api.groq.com` - Groq

可通过环境变量 `API_DOMAINS` 添加更多域名（逗号分隔）。

## ⚙️ 配置

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | 8080 |
| `DATA_DIR` | 数据目录 | ./data |
| `CACHE_TTL_SECONDS` | 缓存 TTL | 43200 (12h) |
| `ADMIN_USERNAME` | 管理员用户名 | admin |
| `ADMIN_PASSWORD` | 管理员密码 | cdnproxy123! |
| `API_DOMAINS` | 额外 API 域名（逗号分隔） | - |
| `WEBP_ENABLED` | 启用 WebP 转换 | false |

### 管理界面

访问 `/admin/` 进行白名单管理（默认账号：`admin` / `cdnproxy123!`）

## 🔒 访问控制

CDN 代理请求会进行访问控制，满足以下任一条件即可通过：

1. **非常见浏览器 User-Agent**（如 curl、wget）
2. **Referer 为 IP 或本地开发**（localhost、127.0.0.1）
3. **域名在白名单中**（在 `/admin/` 管理）

**注意**: API 代理请求不受访问控制限制。

## 📚 文档

- 📖 [使用文档](/docs) - 访问 `/docs` 路由查看
- 📚 [完整文档](docs/zh/README.md) - 中文文档
- 🔄 [更新日志](CHANGELOG.md) - 版本历史

## 🌐 部署

### Railway
1. 连接仓库，选择 Dockerfile 构建
2. 设置 `ADMIN_PASSWORD` 环境变量
3. 部署完成

### Fly.io
```bash
flyctl launch --region hkg
flyctl secrets set ADMIN_PASSWORD="your_password"
flyctl deploy
```

详细指南：[DEPLOY_FLYIO.md](DEPLOY_FLYIO.md)

## 💡 技术特性

- ✅ **无 Redis 依赖**: 使用硬盘文件缓存
- ✅ **数据持久化**: 缓存和配置自动保存
- ✅ **单二进制部署**: 无需额外依赖
- ✅ **智能缓存**: 基于内容类型的缓存策略
- ✅ **流式传输**: 支持大文件和实时流

## 📝 版本历史

- **v2.2**: 移除 Redis 依赖，改用硬盘缓存
- **v2.1**: 新增 API 代理功能，支持 WebSocket 和 SSE
- **v2.0**: 重构架构，优化性能
