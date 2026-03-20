# 更新日志 / Changelog

## v2.3.0-rc3 (2026-03-21)

### 🐛 Bug 修复

- **修复根路径验证问题**：访问如 `/google.com/` 时不再报 "invalid upstream URL" 错误
  - `ValidateURL` 现在正确处理根路径 `/`，允许其通过验证
  - 添加了根路径的测试用例确保行为正确

---

## v2.3.0-rc2 (2026-03-21)

### 🔒 安全增强

- **输入验证**：新增全面的输入验证模块 (`validator.go`)
  - **路径验证**：防止路径遍历攻击 (`../`)、空字节注入、控制字符注入
  - **主机名验证**：验证主机名格式，防止非法字符和格式错误
  - **查询参数验证**：检测 SQL 注入和命令注入模式
  - **URL 验证**：验证协议安全性，只允许 http/https，拒绝危险协议（file://, javascript: 等）
  - **Header 验证**：防止 header 注入攻击，限制 header 数量和大小
  - **Unicode 安全**：防止 Unicode homograph 攻击
- **验证集成**：将输入验证集成到 `handler.go` 的请求处理流程中
  - 在 IP 封禁检查后立即验证所有输入
  - 验证失败时记录日志并返回 400 错误
  - 配合 IP 封禁机制，恶意请求会被自动封禁

### 🧪 测试改进

- **新增验证器测试**：`validator_test.go` 包含 50+ 个测试用例
  - 路径遍历攻击测试
  - SQL 注入检测测试
  - 命令注入检测测试
  - Header 注入测试
  - 边界条件测试
  - 所有测试通过 ✅

### 📝 文档更新

- 更新 CHANGELOG.md，记录输入验证改进

---

## v2.3.0-rc1 (2026-03-20)

### 🔧 代码质量改进

- **安全增强**：移除硬编码默认密码，要求通过环境变量设置管理员密码
  - 如果未设置 `ADMIN_PASSWORD`，系统会生成临时密码并在启动时显示
  - 添加密码强度验证（最少 8 位字符）
- **配置优化**：将魔法数字移到配置文件
  - 新增 `MAX_CONCURRENT_REQUESTS`（最大并发请求数，默认 50）
  - 新增 `MAX_WEBSOCKET_CONNS`（最大 WebSocket 连接数，默认 10）
  - 新增 `LARGE_FILE_THRESHOLD`（大文件阈值，默认 1MB）
  - 新增 `VIDEO_FILE_THRESHOLD`（视频文件缓存阈值，默认 100MB）
  - 新增 `MAX_CACHE_FILE_SIZE`（最大缓存文件大小，默认 100MB）
- **配置验证**：添加 `Config.Validate()` 方法，在启动时验证配置的有效性

### 📦 代码重构

- **拆分 handler.go**：将 `handler.go` 拆分为多个模块
  - 新增 `cache_handler.go`：缓存相关逻辑
  - 新增 `access_control.go`：访问控制和头部处理
- **消除重复代码**：重构 `api_proxy.go`，合并重复的 `dialUpstream` 函数
- **清理未使用代码**：删除 `ai_optimizer.go` 和 `enterprise_security.go` 占位符文件

### ✨ 新增功能

- **环境变量示例**：新增 `.env.example` 文件，列出所有可配置的环境变量
- **单元测试**：为核心功能添加单元测试
  - `config_test.go`：配置加载和验证测试
  - `cache_handler_test.go`：代理核心功能测试
  - 测试覆盖率：配置模块 100%，代理模块核心功能 >60%

### 🔒 安全改进

- **密码安全**：禁止使用弱密码，强制要求至少 8 位字符
- **配置安全**：生产环境必须显式设置敏感配置
- **日志安全**：生成临时密码时记录警告信息

### 📝 文档更新

- 新增 `.env.example` 文件，包含所有环境变量的说明
- 更新配置文档，说明新增的环境变量

---

## v2.2.1-rc2 (2025-03-02)

### ✨ 新增

- **IP 自动封禁**：当某 IP 在统计窗口内触发过多 400/503 错误时自动封禁，封禁后明确返回 403 及封禁说明
- 环境变量：`IP_BAN_ENABLED`、`IP_BAN_THRESHOLD`（默认 30）、`IP_BAN_WINDOW_SEC`（默认 300）、`IP_BAN_DURATION_SEC`（默认 3600）

---

## v2.2.1-rc1 (2025-03-02)

### ✨ 新增

- **llm.txt / llms.txt**：新增 AI 文档路由，供 LLM 快速理解服务用法
- **路径校验**：非 `/docs`、`/llm.txt`、`/llms.txt` 的请求，若首段 path 不像域名（如 `/favicon.ico`、`/robots.txt`），快速返回 400，节省带宽和 CPU

---

## v2.2.0 (2024-12-XX)

### 🎉 版本更新：文档完善和优化

完善了项目文档和使用说明，提升了用户体验。

### ✨ 文档改进

- **README.md 更新**：
  - 添加版本号 v2.2 标识
  - 完善功能特性说明
  - 优化环境变量配置说明
  - 添加访问控制详细说明
  - 添加更多文档链接

- **/docs 路由文档更新**：
  - 完全重写使用说明文档
  - 移除过时的 Redis 相关内容
  - 添加 API 代理使用示例
  - 完善访问控制说明
  - 添加缓存策略说明
  - 添加功能特性介绍

### 🔧 改进

- 优化文档结构和可读性
- 统一版本号标识
- 完善使用示例和说明

### 📝 文档更新

- 更新 `README.md` - 添加 v2.2 版本信息和完整功能说明
- 更新 `internal/docs/public_usage.md` - 完全重写，移除 Redis 相关内容
- 更新 `CHANGELOG.md` - 添加 v2.2.0 版本记录

### 🔄 向后兼容

- ✅ 完全兼容 v2.1.0 的所有功能
- ✅ 不影响现有配置和使用方式
- ✅ 仅文档更新，无代码变更

---

## v2.1.0 (2024-10-09)

### 🎉 重大更新：API 代理功能

增加了对 AI API 服务（OpenAI、Claude、Poe 等）的代理支持，解决国内无法访问这些服务的问题。

### ✨ 新功能

- **API 代理模式**：自动识别 API 域名，使用专门的代理处理器
- **WebSocket 支持**：完整支持 WebSocket 连接，实现实时双向通信
- **SSE 流式支持**：支持 Server-Sent Events，适配 OpenAI 等服务的 stream 模式
- **大数据传输**：支持 POST 大量数据，无大小限制
- **长连接支持**：无超时限制，支持长时间运行的请求
- **智能域名识别**：内置 10+ 主流 AI 服务域名，自动路由

### 🔧 技术改进

- 新增 `internal/proxy/api_proxy.go` - API 代理处理器
- 扩展 `config.Config` - 添加 `APIDomains` 配置
- 优化 `proxy.Handler` - 添加域名识别和路由逻辑
- 支持环境变量 `API_DOMAINS` - 可自定义额外的 API 域名

### 📝 文档更新

- 更新 `README.md` - 添加 API 代理使用说明
- 更新 `docs/README.md` - 完善功能介绍和示例
- 新增 `docs/API_PROXY_GUIDE.md` - 详细的 API 代理使用指南

### 🎯 支持的 API 服务

默认支持以下服务：
- OpenAI (`api.openai.com`)
- Anthropic Claude (`api.anthropic.com`)
- Poe (`poe.com`, `api.poe.com`)
- Google Gemini (`gemini.google.com`)
- Google AI (`generativelanguage.googleapis.com`)
- Cohere (`api.cohere.ai`)
- Together AI (`api.together.xyz`)
- Groq (`api.groq.com`)

### 🔄 向后兼容

- ✅ 完全兼容原有的 CDN 代理功能
- ✅ 不影响现有配置和使用方式
- ✅ 自动识别请求类型（CDN vs API）

### 📖 使用示例

#### CDN 代理（原有功能）
```bash
https://cdnproxy.some.im/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
```

#### API 代理（新功能）
```bash
curl https://cdnproxy.some.im/api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'
```

---

## v2.0.0 (2024-10-01)

### 🎉 重大更新：移除 Redis 依赖

完全移除 Redis 依赖，改用硬盘文件缓存系统。

### ✨ 新功能

- **硬盘缓存系统**：使用文件系统存储缓存，无需额外服务
- **数据持久化**：缓存和配置自动保存到硬盘
- **简化部署**：单个二进制文件即可运行
- **降低成本**：无需维护 Redis 服务

### 🔧 技术改进

- 新增 `internal/cache/disk_cache.go` - 硬盘缓存实现
- 新增 `internal/storage/` - 文件存储系统
- 优化大文件处理：>5MB 文件使用流式传输
- 支持 Range 请求，实现断点续传

### 📝 配置变化

新增环境变量：
- `DATA_DIR` - 数据存储目录（默认 `./data`）
- `CACHE_DIR` - 缓存文件目录（默认 `./data/cache`）

移除环境变量：
- `REDIS_URL` - 不再需要 Redis

---

## v1.0.0 (2024-09-01)

### 🎉 首次发布

- **CDN 代理**：支持代理 jsDelivr, unpkg 等 CDN 资源
- **Redis 缓存**：使用 Redis 存储响应缓存
- **访问控制**：基于 Referer 的白名单机制
- **管理后台**：Web 界面管理白名单
- **WebP 转换**：自动将图片转换为 WebP 格式
- **压缩优化**：自动 gzip 压缩响应

---

## 计划中的功能

### v2.3.0（计划中）
- [ ] API 请求日志和统计
- [ ] API 请求速率限制
- [ ] 支持更多 AI 服务
- [ ] 性能监控面板

### v3.0.0（计划中）
- [ ] 分布式部署支持
- [ ] 高可用集群
- [ ] 自动故障转移
- [ ] 更强大的缓存策略

