# CDNProxy

一个用于在受限网络环境下代理访问公共 CDN 资源并进行 Redis 缓存的 Go 服务，默认端口 8080，适配 Railway 部署。

## 功能

- 基于路径的反向代理：/host/path → https://host/path
- 访问控制策略：
  1. 非常见浏览器 User-Agent 允许
  2. Referer 为 IP 或本地开发（localhost 等）允许
  3. Referer 为域名且后缀在白名单中允许
- Redis 缓存 GET/HEAD 响应（默认 12 小时）
- /admin 管理界面：登录后增删白名单后缀

默认管理员：`admin` / `cdnproxy123!`，可通过环境变量覆盖。

## 路由

- `/healthz` 健康检查
- `/admin/login` 登录页
- `/admin/` 管理台（需登录）

## 环境变量

- `PORT`：服务端口，默认 8080
- `REDIS_URL` 或 `RAILWAY_REDIS_URL`：Redis 连接 URL（如：redis://localhost:6379/0）
- `CACHE_TTL_SECONDS`：缓存 TTL，默认 43200（12h）
- `SESSION_TTL_SECONDS`：管理登录会话 TTL，默认 86400（24h）
- `ADMIN_USERNAME` / `ADMIN_PASSWORD`：管理登录凭据

## 启动

```bash
go mod tidy
REDIS_URL=redis://localhost:6379/0 go run ./...
```

本地测试示例：

```bash
curl -i http://localhost:8080/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

## Railway 部署

支持 Dockerfile 构建，或直接使用 `railway.json`。

步骤：
1. 新建 Railway 项目，添加 Redis 插件（或自备 Redis 服务）
2. 连接本仓库后部署，构建方式选择 Dockerfile（已内置）
3. 环境变量：
   - `PORT=8080`
   - `REDIS_URL`（推荐设置；如未设置，将回退为 `redis://default:fwuVkYeWWlaxeNDUlABLbJRnZmNVLWVw@redis.railway.internal:6379`）
   - `CACHE_TTL_SECONDS=43200`（可选）
   - `SESSION_TTL_SECONDS=86400`（可选）
   - `ADMIN_USERNAME`、`ADMIN_PASSWORD`（可选）
4. 自定义域名：将域名指向 Railway 提供的地址

示例访问：

```
https://cdnproxy.some.im/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

若访问被阻止，请确保：
- 使用非常见浏览器 UA，或
- Referer 为 IP/localhost 开发环境，或
- 将站点域名后缀加入白名单（在 `/admin/` 操作）


