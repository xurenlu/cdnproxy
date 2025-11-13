# CDNProxy v2.2 使用说明

本文档将指导您如何使用 CDNProxy 服务。

## 项目简介

CDNProxy 是一个轻量级的反向代理服务，旨在解决在特定网络环境下访问公共 CDN（如 cdn.jsdelivr.net, unpkg.com）和 AI API 服务（如 OpenAI、Claude）困难的问题。它通过将请求代理到您自己的服务器，并利用**硬盘文件缓存**系统来提高资源的访问速度和稳定性。

### 核心特性

- ✅ **CDN 资源代理**：支持代理主流 CDN 资源
- ✅ **硬盘缓存**：使用文件系统缓存，无需 Redis
- ✅ **API 代理**：支持代理 AI 服务 API（OpenAI、Claude 等）
- ✅ **WebSocket 支持**：完整支持实时双向通信
- ✅ **SSE 流式支持**：支持 Server-Sent Events 流式响应
- ✅ **访问控制**：灵活的访问控制策略
- ✅ **管理后台**：Web 界面管理白名单和配置

## 如何使用

本服务已部署在 `https://cdnproxy.shifen.de`。使用时，只需将原始 URL 中的 `https://` 替换为 `https://cdnproxy.shifen.de/` 即可。

### CDN 资源代理示例

#### 示例 1: `cdn.jsdelivr.net`

**原始 URL:**
```
https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

**通过本代理访问:**
```
https://cdnproxy.shifen.de/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

#### 示例 2: `unpkg.com`

**原始 URL:**
```
https://unpkg.com/react@18/umd/react.development.js
```

**通过本代理访问:**
```
https://cdnproxy.shifen.de/unpkg.com/react@18/umd/react.development.js
```

**使用方法：** 将原始 CDN URL 中的 `https://` 替换为 `https://cdnproxy.shifen.de/` 即可。

### API 服务代理示例

#### OpenAI API 示例

**原始请求：**
```bash
curl https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

**通过代理：**
```bash
curl https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

#### Claude API 示例

**原始 URL:**
```
https://api.anthropic.com/v1/messages
```

**通过代理:**
```
https://cdnproxy.shifen.de/api.anthropic.com/v1/messages
```

#### 支持的 API 服务

默认支持以下 AI 服务：
- `api.openai.com` - OpenAI API
- `api.anthropic.com` - Anthropic Claude API
- `claude.ai` - Claude Web
- `poe.com` / `api.poe.com` - Poe
- `gemini.google.com` - Google Gemini
- `generativelanguage.googleapis.com` - Google AI
- `api.cohere.ai` - Cohere
- `api.together.xyz` - Together AI
- `api.groq.com` - Groq

可通过环境变量 `API_DOMAINS` 添加更多域名（逗号分隔）。

## 访问控制说明

为了防止代理服务被滥用，CDN 代理请求实施了访问控制策略。如果您的请求被意外阻止，通常是因为您访问本服务的网站域名（即 Referer）不在白名单中。

### 访问控制规则

CDN 代理请求会在以下情况下**允许访问**：

1. **非常见浏览器 User-Agent**：使用 curl、wget 或自定义 UA
2. **Referer 为 IP 地址**：如 `http://192.168.1.1/`
3. **Referer 为本地开发环境**：如 `http://localhost:3000/`、`http://127.0.0.1:8080/`
4. **Referer 域名在白名单中**：域名后缀已加入白名单

### API 代理不受限制

**重要**：API 代理请求不受访问控制限制，由上游 API 服务自己控制访问权限。

### 如何加入白名单

如有需要，请联系服务管理员将您的网站域名加入白名单，或使用管理后台（`/admin/`）自行管理。

## 缓存说明

### CDN 资源缓存

- **缓存策略**：GET/HEAD 请求会被缓存
- **缓存时间**：默认 12 小时，根据内容类型自动调整
  - 静态资源（CSS/JS）：7 天
  - 图片资源：1 天
  - 字体文件：30 天
  - HTML 文档：1 小时
- **大文件处理**：≥5MB 文件直接流式传输，不缓存
- **存储方式**：硬盘文件缓存，无需 Redis

### API 响应不缓存

API 代理请求**不会缓存**，每次都是实时请求，确保数据的实时性和准确性。

## 功能特性

### WebSocket 支持

完整支持 WebSocket 连接，实现实时双向通信，适用于需要实时交互的 AI 服务。

### SSE 流式支持

支持 Server-Sent Events（SSE），适配 OpenAI 等服务的 stream 模式，实现流式响应。

### 大数据传输

支持 POST 大量数据，无大小限制，适合上传大文件或发送大量数据。

### 长连接支持

支持长时间运行的请求，无超时限制，适合需要长时间处理的 API 请求。

## 管理后台

访问 `/admin/` 可以管理：
- **白名单管理**：添加/删除允许访问的域名后缀
- **配置管理**：修改系统配置和阈值设置
- **访问统计**：查看访问计数和统计信息

默认管理员账号：`admin` / `cdnproxy123!`（生产环境请务必修改）

## 健康检查

访问 `/healthz` 可以查看服务健康状态，包括：
- 服务状态
- 内存使用情况
- Goroutine 数量
- 运行时间

## 更多信息

- 📚 [完整文档](../docs/zh/README.md) - 查看详细的中文文档
- 🔄 [更新日志](../CHANGELOG.md) - 查看版本更新历史
- 🐛 [问题反馈](https://github.com/your-repo/cdnproxy/issues) - 提交问题或建议

---

**版本**: v2.2 | **最后更新**: 2024-12
