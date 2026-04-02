# 住宅IP代理使用指南

## 🏠 **住宅IP代理功能概述**

### 为什么需要住宅IP代理？

AI服务商（如OpenAI、Claude、Gemini）对IP类型有严格限制：
- ❌ 禁止数据中心IP访问
- ❌ 检测并封禁代理服务
- ❌ 限制商业IP使用
- ✅ 只允许住宅IP和移动IP

### 我们的解决方案

通过集成多个住宅IP代理提供者，实现：
- ✅ 智能代理选择
- ✅ 自动故障转移
- ✅ 成本控制
- ✅ 性能监控

## 🔧 **支持的住宅IP提供者**

### 1. **Bright Data** (推荐)
```
优势:
- 全球最大住宅IP网络
- 高质量IP资源
- 稳定可靠
- 支持多种协议

成本:
- 请求费用: $0.001/次
- 流量费用: $0.50/GB
- 时间费用: $0.10/小时

配置:
export BRIGHT_DATA_API_KEY="your_api_key"
export BRIGHT_DATA_USERNAME="your_username"
export BRIGHT_DATA_PASSWORD="your_password"
```

### 2. **Smartproxy**
```
优势:
- 价格相对便宜
- 易于使用
- 支持多种集成方式
- 良好的客户支持

成本:
- 请求费用: $0.0008/次
- 流量费用: $0.40/GB
- 时间费用: $0.08/小时

配置:
export SMARTPROXY_API_KEY="your_api_key"
export SMARTPROXY_USERNAME="your_username"
export SMARTPROXY_PASSWORD="your_password"
```

### 3. **NetNut**
```
优势:
- 快速稳定
- 全球覆盖
- 支持大规模使用
- 良好的技术支持

成本:
- 请求费用: $0.0012/次
- 流量费用: $0.60/GB
- 时间费用: $0.12/小时

配置:
export NETNUT_API_KEY="your_api_key"
export NETNUT_USERNAME="your_username"
export NETNUT_PASSWORD="your_password"
```

## 🚀 **快速开始**

### 1. **环境配置**
```bash
# 设置至少一个住宅IP提供者的API密钥
export BRIGHT_DATA_API_KEY="your_api_key"
export BRIGHT_DATA_USERNAME="your_username"
export BRIGHT_DATA_PASSWORD="your_password"

# 或者使用Smartproxy
export SMARTPROXY_API_KEY="your_api_key"
export SMARTPROXY_USERNAME="your_username"
export SMARTPROXY_PASSWORD="your_password"

# 或者使用NetNut
export NETNUT_API_KEY="your_api_key"
export NETNUT_USERNAME="your_username"
export NETNUT_PASSWORD="your_password"
```

### 2. **启动服务**
```bash
# 编译项目
go build -o cdnproxy

# 启动服务
PORT=8081 ./cdnproxy
```

### 3. **测试住宅IP代理**
```bash
# 运行测试脚本
./scripts/test_residential_proxy.sh
```

## 📊 **使用示例**

### 1. **OpenAI API代理**
```bash
# 通过住宅IP代理访问OpenAI API
curl "https://cdnproxy.facev.app/api.openai.com/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_OPENAI_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 2. **Claude API代理**
```bash
# 通过住宅IP代理访问Claude API
curl "https://cdnproxy.facev.app/api.anthropic.com/v1/messages" \
  -H "x-api-key: YOUR_CLAUDE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus-20240229",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 3. **Gemini API代理**
```bash
# 通过住宅IP代理访问Gemini API
curl "https://cdnproxy.facev.app/generativelanguage.googleapis.com/v1beta/models" \
  -H "Authorization: Bearer YOUR_GEMINI_KEY"
```

## 🔍 **监控和管理**

### 1. **代理统计**
```bash
# 查看代理统计信息
curl "http://localhost:8081/admin/proxy/stats"
```

### 2. **健康状态**
```bash
# 查看代理健康状态
curl "http://localhost:8081/admin/proxy/health"
```

### 3. **成本统计**
```bash
# 查看成本统计
curl "http://localhost:8081/admin/proxy/cost"
```

## ⚙️ **配置选项**

### 1. **代理池配置**
```yaml
# config/residential_proxy.yml
proxy_pool:
  health_check:
    interval: 30s
    timeout: 10s
    max_failures: 3
  
  selection:
    quality_weight: 40
    success_rate_weight: 30
    location_weight: 20
    usage_weight: 10
```

### 2. **成本控制**
```yaml
cost_control:
  daily_max_cost: 100.0
  monthly_max_cost: 3000.0
  cost_alert_threshold: 0.8
```

### 3. **地理位置偏好**
```yaml
location_preferences:
  openai: ["US", "CA", "GB", "AU"]
  claude: ["US", "CA", "GB", "AU"]
  gemini: ["US", "CA", "GB", "AU"]
```

## 📈 **性能优化**

### 1. **智能代理选择**
- 基于质量评分选择最佳代理
- 考虑成功率、延迟、地理位置
- 自动负载均衡

### 2. **故障转移**
- 自动检测代理故障
- 快速切换到备用代理
- 健康检查和自动恢复

### 3. **成本优化**
- 实时成本监控
- 自动成本控制
- 成本告警机制

## 🛡️ **安全考虑**

### 1. **API密钥安全**
- 使用环境变量存储API密钥
- 不在代码中硬编码密钥
- 定期轮换API密钥

### 2. **请求头伪装**
- 模拟真实浏览器请求
- 随机化User-Agent
- 添加真实请求头

### 3. **频率控制**
- 模拟人类请求模式
- 随机延迟和间隔
- 避免被检测为机器人

## 💰 **成本管理**

### 1. **成本监控**
```bash
# 查看实时成本
curl "http://localhost:8081/admin/proxy/cost"

# 查看成本历史
curl "http://localhost:8081/admin/proxy/cost/history"
```

### 2. **成本控制**
- 设置每日/每月成本限制
- 自动成本告警
- 成本超限自动停止

### 3. **成本优化**
- 选择性价比最高的提供者
- 优化代理使用策略
- 减少不必要的请求

## 🔧 **故障排查**

### 1. **常见问题**

#### 代理连接失败
```bash
# 检查代理健康状态
curl "http://localhost:8081/admin/proxy/health"

# 检查API密钥配置
echo $BRIGHT_DATA_API_KEY
```

#### 请求被拒绝
```bash
# 检查请求头
curl -v "http://localhost:8081/api.openai.com/v1/models"

# 检查代理质量
curl "http://localhost:8081/admin/proxy/stats"
```

#### 成本超限
```bash
# 检查成本统计
curl "http://localhost:8081/admin/proxy/cost"

# 调整成本限制
# 编辑 config/residential_proxy.yml
```

### 2. **日志分析**
```bash
# 查看服务日志
tail -f logs/cdnproxy.log

# 查看代理日志
tail -f logs/proxy.log
```

### 3. **性能分析**
```bash
# 查看性能指标
curl "http://localhost:8081/metrics"

# 查看详细统计
curl "http://localhost:8081/stats"
```

## 📚 **最佳实践**

### 1. **代理选择**
- 优先选择高质量代理
- 考虑地理位置匹配
- 监控成功率统计

### 2. **成本控制**
- 设置合理的成本限制
- 定期检查成本统计
- 优化使用策略

### 3. **性能优化**
- 启用健康检查
- 配置故障转移
- 监控延迟和成功率

### 4. **安全防护**
- 保护API密钥安全
- 使用HTTPS传输
- 定期更新配置

## 🎯 **总结**

通过住宅IP代理功能，我们可以：
- ✅ 成功访问被限制的AI API
- ✅ 提供稳定的代理服务
- ✅ 控制使用成本
- ✅ 监控服务质量

**关键成功因素：**
1. 选择合适的住宅IP提供者
2. 正确配置API密钥
3. 监控代理性能
4. 控制使用成本

**现在你可以安全地使用住宅IP代理访问OpenAI、Claude、Gemini等AI服务了！** 🚀
