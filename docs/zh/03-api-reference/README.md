# API 参考

CDNProxy API 参考文档。

## 📖 **内容**

### [住宅IP代理API](residential-proxy-api.md)
住宅IP代理相关 API：
- 代理统计 API
- 健康检查 API
- 提供者列表 API
- 使用示例

## 🔧 **API 端点**

### 管理 API

- `GET /admin/proxy/stats` - 获取代理统计
- `GET /admin/proxy/health` - 获取健康状态
- `GET /admin/proxy/providers` - 获取提供者列表

### 监控 API

- `GET /metrics` - Prometheus 格式指标
- `GET /stats` - JSON 格式统计
- `GET /healthz` - 健康检查

## 📚 **相关文档**

- [用户指南](../02-user-guide/)
- [部署运维](../04-deployment/)
- [故障排查](../06-troubleshooting/)
