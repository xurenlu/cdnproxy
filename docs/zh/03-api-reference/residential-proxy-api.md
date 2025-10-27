# 住宅IP代理 API 文档

## 📚 **概述**

本文档介绍 CDNProxy 的住宅IP代理功能 API，包括配置、使用和管理。

## 🔧 **配置**

### 环境变量

#### Bright Data
```bash
export BRIGHT_DATA_API_KEY="your_api_key"
export BRIGHT_DATA_USERNAME="your_username"
export BRIGHT_DATA_PASSWORD="your_password"
```

#### Smartproxy
```bash
export SMARTPROXY_API_KEY="your_api_key"
export SMARTPROXY_USERNAME="your_username"
export SMARTPROXY_PASSWORD="your_password"
```

#### NetNut
```bash
export NETNUT_API_KEY="your_api_key"
export NETNUT_USERNAME="your_username"
export NETNUT_PASSWORD="your_password"
```

#### Oxylabs
```bash
export OXYLABS_API_KEY="your_api_key"
export OXYLABS_USERNAME="your_username"
export OXYLABS_PASSWORD="your_password"
```

#### Proxy-Seller
```bash
export PROXY_SELLER_API_KEY="your_api_key"
export PROXY_SELLER_USERNAME="your_username"
export PROXY_SELLER_PASSWORD="your_password"
```

#### Youproxy
```bash
export YOUPROXY_API_KEY="your_api_key"
export YOUPROXY_USERNAME="your_username"
export YOUPROXY_PASSWORD="your_password"
```

#### Geonix
```bash
export GEONIX_API_KEY="your_api_key"
export GEONIX_USERNAME="your_username"
export GEONIX_PASSWORD="your_password"
```

#### IPBurger
```bash
export IPBURGER_API_KEY="your_api_key"
export IPBURGER_USERNAME="your_username"
export IPBURGER_PASSWORD="your_password"
```

## 🚀 **使用示例**

### 1. OpenAI API 代理

```bash
curl "https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_OPENAI_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 2. Claude API 代理

```bash
curl "https://cdnproxy.shifen.de/api.anthropic.com/v1/messages" \
  -H "x-api-key: YOUR_CLAUDE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus-20240229",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 3. Gemini API 代理

```bash
curl "https://cdnproxy.shifen.de/generativelanguage.googleapis.com/v1beta/models" \
  -H "Authorization: Bearer YOUR_GEMINI_KEY"
```

### 4. Python 示例

```python
import requests

# OpenAI API
response = requests.post(
    "https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions",
    headers={
        "Authorization": "Bearer YOUR_OPENAI_KEY",
        "Content-Type": "application/json"
    },
    json={
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello!"}]
    }
)
print(response.json())
```

### 5. JavaScript 示例

```javascript
// OpenAI API
fetch('https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_OPENAI_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    model: 'gpt-4',
    messages: [{role: 'user', content: 'Hello!'}]
  })
})
.then(response => response.json())
.then(data => console.log(data));
```

## 📊 **管理 API**

### 1. 代理统计

```bash
curl "http://localhost:8081/admin/proxy/stats"
```

响应示例：
```json
{
  "provider_count": 6,
  "healthy_proxies": 50,
  "unhealthy_proxies": 5,
  "average_latency": 1250
}
```

### 2. 健康检查

```bash
curl "http://localhost:8081/admin/proxy/health"
```

响应示例：
```json
{
  "total_proxies": 55,
  "healthy_proxies": 50,
  "health_rate": 90.91
}
```

### 3. 提供者列表

```bash
curl "http://localhost:8081/admin/proxy/providers"
```

响应示例：
```json
{
  "providers": ["bright_data", "smartproxy", "netnut", "oxylabs", "proxy_seller", "youproxy"],
  "count": 6
}
```

## 🎯 **最佳实践**

### 1. 配置多个提供者

建议配置多个提供者以提供冗余和故障转移：

```bash
# 配置至少2-3个提供者
export BRIGHT_DATA_API_KEY="your_key"
export BRIGHT_DATA_USERNAME="your_username"
export BRIGHT_DATA_PASSWORD="your_password"

export SMARTPROXY_API_KEY="your_key"
export SMARTPROXY_USERNAME="your_username"
export SMARTPROXY_PASSWORD="your_password"
```

### 2. 监控成本

定期检查使用成本：

```bash
curl "http://localhost:8081/admin/proxy/stats" | jq '.total_cost'
```

### 3. 健康检查

定期检查代理健康状态：

```bash
curl "http://localhost:8081/admin/proxy/health" | jq '.health_rate'
```

### 4. 性能优化

根据使用情况调整配置：

- 低成本场景：使用 Proxy-Seller, Youproxy
- 高质量场景：使用 Bright Data, Oxylabs
- 平衡场景：使用 Smartproxy, NetNut

## 🔍 **故障排查**

### 1. 代理不可用

```bash
# 检查环境变量
echo $BRIGHT_DATA_API_KEY

# 检查健康状态
curl "http://localhost:8081/admin/proxy/health"
```

### 2. 成本超限

```bash
# 检查总成本
curl "http://localhost:8081/admin/proxy/stats" | jq '.total_cost'

# 调整预算限制
# 编辑 config/residential_proxy.yml
```

### 3. 延迟过高

```bash
# 检查平均延迟
curl "http://localhost:8081/admin/proxy/stats" | jq '.average_latency'

# 切换到更快的提供者
```

## 📈 **性能指标**

### 成功率
- 目标: >95%
- 监控: `/admin/proxy/stats`

### 平均延迟
- 目标: <2000ms
- 监控: `/admin/proxy/stats`

### 健康率
- 目标: >90%
- 监控: `/admin/proxy/health`

### 成本
- 监控: `/admin/proxy/stats`
- 告警: 接近预算限制时

## 🎉 **总结**

通过配置住宅IP代理，您可以：
- ✅ 成功访问被限制的AI API
- ✅ 自动选择最佳代理
- ✅ 监控使用成本
- ✅ 优化性能和质量

**现在您可以安全地使用住宅IP代理访问 OpenAI、Claude、Gemini 等 AI 服务了！** 🚀
