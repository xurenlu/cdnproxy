# API 代理使用指南

本文档详细介绍如何使用 CDNProxy 代理 AI API 服务（如 OpenAI、Claude、Poe 等）。

## 功能特性

✅ **支持的协议和功能：**
- HTTP/HTTPS 所有方法（GET、POST、PUT、DELETE、PATCH 等）
- WebSocket 连接（实时双向通信）
- SSE (Server-Sent Events) 流式响应
- 大文件上传/下载（无大小限制）
- 长时间连接（无超时限制）

✅ **优势：**
- **不缓存** API 响应（每次都是实时数据）
- **无访问控制** 限制（不需要加白名单）
- **完整转发** 所有请求头和响应头
- **自动识别** API 域名，无需额外配置

## 支持的 API 服务

默认支持以下 AI 服务的 API 域名：

| 服务 | 域名 | 说明 |
|------|------|------|
| OpenAI | `api.openai.com` | ChatGPT、GPT-4 等 |
| Anthropic | `api.anthropic.com` | Claude API |
| Claude Web | `claude.ai` | Claude 网页版 |
| Poe | `poe.com`, `api.poe.com` | Poe 平台 |
| Google Gemini | `gemini.google.com` | Gemini API |
| Google AI | `generativelanguage.googleapis.com` | Google AI Studio |
| Cohere | `api.cohere.ai` | Cohere API |
| Together AI | `api.together.xyz` | Together AI |
| Groq | `api.groq.com` | Groq API |

### 添加自定义 API 域名

通过环境变量 `API_DOMAINS` 添加额外的域名（逗号分隔）：

```bash
export API_DOMAINS="api.example.com,api2.example.com"
./cdnproxy
```

## 使用方法

### 基本原理

将原始 API URL 中的 `https://` 替换为 `https://cdnproxy.shifen.de/` 即可。

**格式：**
```
原始: https://api.example.com/v1/endpoint
代理: https://cdnproxy.shifen.de/api.example.com/v1/endpoint
```

## 使用示例

### 1. OpenAI API

#### 1.1 非流式请求

```bash
curl https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

#### 1.2 流式请求 (SSE)

```bash
curl https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "讲个笑话"}],
    "stream": true
  }'
```

#### 1.3 Python 代码示例

```python
import openai

# 方法1: 修改 api_base
openai.api_base = "https://cdnproxy.shifen.de/api.openai.com/v1"
openai.api_key = "YOUR_API_KEY"

# 方法2: 使用新版 OpenAI SDK
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://cdnproxy.shifen.de/api.openai.com/v1"
)

# 非流式
response = client.chat.completions.create(
    model="gpt-4",
    messages=[
        {"role": "user", "content": "你好"}
    ]
)
print(response.choices[0].message.content)

# 流式
stream = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "讲个故事"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

#### 1.4 JavaScript/TypeScript 示例

```javascript
// Node.js
const { Configuration, OpenAIApi } = require('openai');

const configuration = new Configuration({
  apiKey: 'YOUR_API_KEY',
  basePath: 'https://cdnproxy.shifen.de/api.openai.com/v1'
});

const openai = new OpenAIApi(configuration);

async function chat() {
  const response = await openai.createChatCompletion({
    model: 'gpt-4',
    messages: [{ role: 'user', content: '你好' }],
    stream: true
  });
  
  console.log(response.data);
}

chat();
```

### 2. Claude API (Anthropic)

#### 2.1 基本请求

```bash
curl https://cdnproxy.shifen.de/api.anthropic.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-opus-20240229",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello, Claude!"}
    ]
  }'
```

#### 2.2 流式请求

```bash
curl https://cdnproxy.shifen.de/api.anthropic.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-opus-20240229",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "讲个故事"}],
    "stream": true
  }'
```

#### 2.3 Python 示例

```python
import anthropic

client = anthropic.Anthropic(
    api_key="YOUR_API_KEY",
    base_url="https://cdnproxy.shifen.de/api.anthropic.com"
)

# 非流式
message = client.messages.create(
    model="claude-3-opus-20240229",
    max_tokens=1024,
    messages=[
        {"role": "user", "content": "你好，Claude！"}
    ]
)
print(message.content)

# 流式
with client.messages.stream(
    model="claude-3-opus-20240229",
    max_tokens=1024,
    messages=[{"role": "user", "content": "讲个故事"}]
) as stream:
    for text in stream.text_stream:
        print(text, end="")
```

### 3. Google Gemini API

```bash
curl https://cdnproxy.shifen.de/generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=YOUR_API_KEY \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{
      "parts": [{
        "text": "Explain how AI works"
      }]
    }]
  }'
```

### 4. Groq API

```bash
curl https://cdnproxy.shifen.de/api.groq.com/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "mixtral-8x7b-32768",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

## WebSocket 支持

对于需要 WebSocket 连接的 API，代理会自动识别并处理升级请求。

### 示例：Poe WebSocket

```javascript
const WebSocket = require('ws');

const ws = new WebSocket('wss://cdnproxy.shifen.de/poe.com/api/chat', {
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'Origin': 'https://poe.com'
  }
});

ws.on('open', function open() {
  console.log('连接已建立');
  ws.send(JSON.stringify({ type: 'subscribe', channel: 'chat' }));
});

ws.on('message', function message(data) {
  console.log('收到消息:', data);
});
```

## 常见问题

### Q1: 为什么 API 请求不需要加白名单？

A: API 代理模式专门为 AI 服务设计，不应用访问控制策略。访问控制由上游 API 服务自己的认证机制（API Key、Token 等）来处理。

### Q2: API 响应会被缓存吗？

A: 不会。所有识别为 API 域名的请求都不会被缓存，确保每次都是实时数据。

### Q3: 支持上传大文件吗？

A: 支持。API 代理模式对请求和响应的大小没有限制，可以上传和下载大文件。

### Q4: 流式响应（SSE）会断开吗？

A: 不会。代理服务器配置为无超时限制，支持长时间的流式响应。

### Q5: 可以同时使用 CDN 代理和 API 代理吗？

A: 可以。同一个服务同时支持两种模式，根据域名自动识别：
- API 域名 → API 代理模式（不缓存、支持 WebSocket/SSE）
- 其他域名 → CDN 代理模式（缓存、访问控制）

### Q6: 如何添加新的 API 域名？

A: 通过环境变量 `API_DOMAINS` 添加：

```bash
export API_DOMAINS="api.newservice.com,api2.example.com"
./cdnproxy
```

或在 Docker 中：

```yaml
environment:
  - API_DOMAINS=api.newservice.com,api2.example.com
```

## 性能优化建议

1. **使用 HTTP/2**：代理服务器支持 HTTP/2，提高并发性能
2. **复用连接**：使用连接池，避免频繁建立新连接
3. **流式处理**：对于长响应，使用流式处理减少内存占用
4. **并发请求**：代理服务器支持高并发，可以同时发起多个请求

## 安全建议

1. **不要在 URL 中暴露 API Key**：始终使用请求头传递认证信息
2. **使用 HTTPS**：确保使用 `https://` 访问代理服务
3. **定期更新**：及时更新代理服务到最新版本
4. **监控使用**：通过日志监控 API 使用情况

## 技术实现

### 处理流程

1. **请求到达** → 解析 URL
2. **域名识别** → 检查是否为 API 域名
3. **路由选择** → API 域名使用 API 代理处理器
4. **协议检查** → 识别 WebSocket/SSE/普通 HTTP
5. **请求转发** → 完整转发所有头和 body
6. **响应处理** → 流式传输响应（不缓存）

### 关键特性

- **零缓存**：API 响应不经过缓存层
- **完整转发**：保留所有请求头和响应头
- **流式传输**：边接收边转发，减少延迟
- **连接复用**：使用连接池提高性能
- **无超时**：支持长时间运行的请求

## 故障排查

### 问题：连接超时

**原因**：可能是网络问题或上游 API 服务不可用

**解决**：
1. 检查网络连接
2. 尝试直接访问上游 API（不通过代理）
3. 查看代理服务器日志

### 问题：流式响应中断

**原因**：可能是客户端或中间网络设备超时

**解决**：
1. 确保客户端没有设置超时
2. 检查是否有反向代理或负载均衡器限制
3. 增加 keep-alive 时间

### 问题：WebSocket 连接失败

**原因**：可能是缺少必要的头信息

**解决**：
1. 确保包含 `Upgrade: websocket` 头
2. 确保包含 `Connection: Upgrade` 头
3. 检查 `Sec-WebSocket-Key` 是否存在

## 更新日志

### v2.1.0 (2024-10-09)
- ✨ 新增 API 代理功能
- ✨ 支持 WebSocket 连接
- ✨ 支持 SSE 流式响应
- ✨ 支持无限大小的请求/响应
- ✨ 自动识别 API 域名
- ✨ 配置 10+ 主流 AI 服务域名

## 相关文档

- [主文档](../README.md)
- [部署指南](DEPLOYMENT_GUIDE.md)
- [性能优化](PERFORMANCE_OPTIMIZATION.md)

