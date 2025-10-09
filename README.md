# CDNProxy

一个用于在受限网络环境下代理访问公共 CDN 资源并进行**硬盘缓存**的 Go 服务，默认端口 8080。

## ✨ v2.0 重大更新

- **完全移除 Redis 依赖**，改用硬盘文件缓存系统
- **简化部署**，无需额外的 Redis 服务
- **数据持久化**，缓存和配置自动保存到硬盘
- **降低成本**，单个二进制文件即可运行

详细说明请查看 [README_DISK_CACHE.md](README_DISK_CACHE.md)

## 功能

### CDN 代理功能
- 基于路径的反向代理：/host/path → https://host/path
- 访问控制策略：
  1. 非常见浏览器 User-Agent 允许
  2. Referer 为 IP 或本地开发（localhost 等）允许
  3. Referer 为域名且后缀在白名单中允许
- **硬盘缓存** GET/HEAD 响应（默认 12 小时）
- 大文件流式传输（>5MB），支持 Range 请求
- /admin 管理界面：登录后增删白名单后缀

### API 代理功能 (v2.1 新增)
- 支持代理 OpenAI、Claude、Poe 等 AI 服务的 API 请求
- **不缓存** API 响应（每次都是实时请求）
- 支持 **WebSocket** 连接（实时双向通信）
- 支持 **SSE (Server-Sent Events)** 流式响应（如 OpenAI 的 stream 模式）
- 支持 **POST 大数据**（无大小限制）
- 支持**长时间连接**（无超时限制）
- 自动识别 API 域名，无需额外配置
- API 请求不受访问控制限制（由上游 API 服务自己控制）

默认管理员：`admin` / `cdnproxy123!`，可通过环境变量覆盖。

## 路由

- `/healthz` 健康检查
- `/admin/login` 登录页
- `/admin/` 管理台（需登录）

## 环境变量

- `PORT`：服务端口，默认 8080
- `DATA_DIR`：数据存储目录，默认 ./data
- `CACHE_DIR`：缓存文件目录，默认 ./data/cache
- `CACHE_TTL_SECONDS`：缓存 TTL，默认 43200（12h）
- `SESSION_TTL_SECONDS`：管理登录会话 TTL，默认 86400（24h）
- `ADMIN_USERNAME` / `ADMIN_PASSWORD`：管理登录凭据
- `API_DOMAINS`：额外的 API 域名（逗号分隔），如 `api.example.com,api2.example.com`

## 启动

```bash
# 编译
go build -o cdnproxy .

# 运行（会自动创建 data 目录）
./cdnproxy
```

或直接运行：

```bash
go mod tidy
go run ./...
```

本地测试示例：

```bash
# 本地开发测试
curl -i http://localhost:8080/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css

# 使用线上服务测试
curl -i https://cdnproxy.shifen.de/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

## Docker 部署

```bash
# 使用 docker-compose（推荐）
docker-compose up -d

# 或手动运行
docker build -t cdnproxy .
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e ADMIN_PASSWORD=your_password \
  cdnproxy
```

**重要**：使用 `-v` 挂载数据目录以持久化缓存！

## Railway 部署

支持 Dockerfile 构建，或直接使用 `railway.json`。

步骤：
1. 新建 Railway 项目
2. 连接本仓库后部署，构建方式选择 Dockerfile
3. 设置环境变量（Railway 会自动挂载持久化卷）：
   - `ADMIN_PASSWORD`（必须修改！）
   - 其他可选配置见上方
4. 自定义域名：将域名指向 Railway 提供的地址

## Fly.io 部署（推荐亚洲节点）

```bash
# 安装 flyctl
brew install flyctl

# 登录并部署
flyctl auth login
flyctl launch --region hkg  # 香港节点
flyctl secrets set ADMIN_PASSWORD="your_password"
flyctl deploy
```

详细指南：[DEPLOY_FLYIO.md](DEPLOY_FLYIO.md)

## 使用示例

本项目已部署在 `https://cdnproxy.shifen.de`，可直接使用。

### CDN 资源代理

**原始 URL:**
```
https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

**通过本代理访问:**
```
https://cdnproxy.shifen.de/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

**使用方法：** 将原始 CDN URL 中的 `https://` 替换为 `https://cdnproxy.shifen.de/` 即可。

### API 服务代理

**OpenAI API 示例:**

原始请求：
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

通过代理：
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

**Claude API 示例:**

```bash
# 原始: https://api.anthropic.com/v1/messages
# 代理: https://cdnproxy.shifen.de/api.anthropic.com/v1/messages
```

**支持的 API 域名：**
- `api.openai.com` - OpenAI API
- `api.anthropic.com` - Claude API
- `claude.ai` - Claude Web
- `poe.com` / `api.poe.com` - Poe
- `gemini.google.com` - Google Gemini
- `generativelanguage.googleapis.com` - Google AI
- `api.cohere.ai` - Cohere
- `api.together.xyz` - Together AI
- `api.groq.com` - Groq

可通过环境变量 `API_DOMAINS` 添加更多域名（逗号分隔）。

若访问被阻止，请确保：
- 使用非常见浏览器 UA，或
- Referer 为 IP/localhost 开发环境，或
- 将站点域名后缀加入白名单（在 `/admin/` 操作）


