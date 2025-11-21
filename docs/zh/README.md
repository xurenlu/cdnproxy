# CDNProxy 中文文档

## 📚 文档导航

### 🚀 快速开始
- **[快速开始](01-getting-started/README.md)** - 安装、配置、快速上手

### 📖 用户指南
- **[API 代理使用](02-user-guide/api-proxy.md)** - AI API 代理使用方法
- **[住宅 IP 代理](02-user-guide/residential-proxy.md)** - 住宅 IP 代理配置
- **[视频支持](02-user-guide/video-support.md)** - 视频流代理支持
- **[扩展提供者](02-user-guide/extended-providers.md)** - 添加自定义代理提供者

### 🔧 API 参考
- **[住宅 IP 代理 API](03-api-reference/residential-proxy-api.md)** - API 接口文档

### 🚢 部署运维
- **[Docker 部署](04-deployment/docker.md)** - Docker 容器部署
- **[云函数部署](04-deployment/serverless.md)** - Serverless 部署
- **[运维指南](04-deployment/operations.md)** - 运维和监控

### ⚡ 高级功能
- **[性能优化](05-advanced/performance.md)** - 性能调优指南
- **[稳定性修复](05-advanced/stability-fixes.md)** - 稳定性改进

### 🔍 故障排查
- **[关键问题修复](06-troubleshooting/critical-issues.md)** - 常见问题解决
- **[性能问题](06-troubleshooting/critical-performance.md)** - 性能问题排查

### 🏗️ 架构设计
- **[架构文档](07-architecture/ARCHITECTURE.md)** - 系统架构说明

## 🎯 快速查找

根据需求快速定位：

| 需求 | 文档 |
|------|------|
| 首次使用 | [快速开始](01-getting-started/README.md) |
| 代理 AI API | [API 代理使用](02-user-guide/api-proxy.md) |
| 配置住宅 IP | [住宅 IP 代理](02-user-guide/residential-proxy.md) |
| Docker 部署 | [Docker 部署](04-deployment/docker.md) |
| 性能优化 | [性能优化](05-advanced/performance.md) |
| 遇到问题 | [故障排查](06-troubleshooting/critical-issues.md) |

## 📋 核心概念

### CDN 代理
将 CDN 资源 URL 的域名部分替换为代理域名即可访问：
```
原始: https://cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
代理: https://your-proxy/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
```

### API 代理
同样替换域名，支持所有 HTTP 方法、WebSocket 和 SSE：
```
原始: https://api.openai.com/v1/chat/completions
代理: https://your-proxy/api.openai.com/v1/chat/completions
```

### 访问控制
- CDN 代理：需要满足 User-Agent、Referer 或白名单条件
- API 代理：不受访问控制限制

### 缓存策略
- CDN 资源：默认缓存 12 小时
- API 请求：不缓存，实时请求

## 🔗 相关链接

- [English Documentation](../en/README.md)
- [GitHub Repository](https://github.com/xurenlu/cdnproxy)
- [更新日志](../../CHANGELOG.md)
