# CDNProxy 使用说明

本文档将指导您如何部署、配置和使用 CDNProxy 服务。

## 目录

- [项目简介](#项目简介)
- [核心功能](#核心功能)
- [部署方式](#部署方式)
  - [本地运行](#本地运行)
  - [Docker 部署](#docker-部署)
  - [Railway 部署](#railway-部署)
- [配置选项](#配置选项)
- [管理后台](#管理后台)
- [使用示例](#使用示例)
- [访问控制详解](#访问控制详解)

## 项目简介

CDNProxy 是一个轻量级的反向代理服务，旨在解决在特定网络环境下访问公共 CDN（如 cdn.jsdelivr.net, unpkg.com）困难的问题。它通过将 CDN 请求代理到您自己的服务器，并利用 Redis 进行响应缓存，来提高资源的访问速度和稳定性。

## 核心功能

- **CDN 代理**: 将形如 `/cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.bundle.min.js` 的请求，智能地代理到 `https://cdn.jsdelivr.net/npm/bootstrap@5/dist/js/bootstrap.bundle.min.js`。
- **Redis 缓存**: 对 `GET` 和 `HEAD` 请求的响应进行缓存，大幅提升重复访问的速度，减少对源 CDN 的请求。缓存时间可配置。
- **访问控制**: 内置灵活的访问控制策略，防止代理服务被滥用。策略包括 User-Agent、Referer IP 和 Referer 域名白名单等。
- **管理后台**: 提供一个简单的 Web 界面，用于管理 Referer 白名单和查看基本设置。

## 部署方式

### 本地运行

适用于开发和快速测试。

**环境要求:**
- Go (1.18+)
- Redis 服务

**步骤:**

1.  克隆代码仓库：
    ```bash
    git clone https://github.com/your-repo/cdnproxy.git
    cd cdnproxy
    ```

2.  安装依赖：
    ```bash
    go mod tidy
    ```

3.  设置环境变量并运行：
    确保你的 Redis 服务正在运行，并将其连接地址设置为环境变量。
    ```bash
    export REDIS_URL="redis://localhost:6379/0"
    go run ./...
    ```

4.  服务启动后，默认监听在 `8080` 端口。

### Docker 部署

推荐用于生产环境，方便快捷。

**环境要求:**
- Docker
- Docker Compose (可选, 推荐)
- Redis 服务 (可与应用一同部署)

**步骤:**

1.  在项目根目录，有一个 `Dockerfile` 文件已经为您准备好。

2.  创建一个 `docker-compose.yml` 文件来编排应用和 Redis 服务：
    ```yaml
    version: '3.8'
    services:
      redis:
        image: redis:6-alpine
        restart: always
        volumes:
          - redis_data:/data

      cdnproxy:
        build: .
        restart: always
        ports:
          - "8080:8080"
        environment:
          - PORT=8080
          - REDIS_URL=redis://redis:6379/0
          # 可选：设置管理员凭据
          - ADMIN_USERNAME=admin
          - ADMIN_PASSWORD=your_strong_password
        depends_on:
          - redis

    volumes:
      redis_data:
    ```

3.  启动服务：
    ```bash
    docker-compose up -d
    ```

### Railway 部署

项目已对 [Railway](https://railway.app/) 平台进行适配，可实现零配置一键部署。

详细步骤请参考项目根目录的 `README.md` 文件。

## 配置选项

您可以通过环境变量来配置 CDNProxy 的行为。

| 环境变量              | 描述                                     | 默认值                                             |
| --------------------- | ---------------------------------------- | -------------------------------------------------- |
| `PORT`                | 服务监听的端口。                         | `8080`                                             |
| `REDIS_URL`           | Redis 连接字符串。                       | `redis://localhost:6379/0` (Railway 会自动注入)    |
| `CACHE_TTL_SECONDS`     | CDN 资源的缓存时间（秒）。                 | `43200` (12 小时)                                  |
| `SESSION_TTL_SECONDS`   | 管理后台登录会话的有效期（秒）。         | `86400` (24 小时)                                  |
| `ADMIN_USERNAME`      | 管理后台的登录用户名。                   | `admin`                                            |
| `ADMIN_PASSWORD`      | 管理后台的登录密码。                     | `cdnproxy123!`                                     |
| `DAILY_REQUEST_LIMIT` | 单个 Referer 域名每日请求次数上限。        | `1000`                                             |

## 管理后台

服务提供了一个简单的管理后台，用于操作白名单和查看配置。

- **访问地址**: `http://<your-proxy-domain>/admin`
- **默认凭据**:
  - 用户名: `admin`
  - 密码: `cdnproxy123!`

登录后，您可以：
- 添加新的域名后缀到白名单 (例如, `example.com`)。
- 从白名单中移除已有的域名后缀。
- 查看当前的配置信息，如缓存 TTL、每日请求上限等。

## 使用示例

本项目已部署在 `https://cdnproxy.shifen.de`，可直接使用。

**原始 URL:**
```
https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

**通过本代理访问:**
```
https://cdnproxy.shifen.de/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

**原始 URL:**
```
https://unpkg.com/react@18/umd/react.development.js
```

**通过本代理访问:**
```
https://cdnproxy.shifen.de/unpkg.com/react@18/umd/react.development.js
```

**使用方法：** 将原始 CDN URL 中的 `https://` 替换为 `https://cdnproxy.shifen.de/` 即可。

## 访问控制详解

为了防止代理服务被滥用，CDNProxy 实施了以下访问控制策略。只有满足 **任意一条** 规则的请求才会被处理，否则将被拒绝。

1.  **User-Agent 检查**:
    - 如果请求的 `User-Agent` 不是常见的浏览器 UA (如 `curl`, `wget`, `python-requests` 等)，请求将被 **允许**。这是为了方便开发者进行测试。

2.  **Referer 检查**:
    - 如果请求的 `Referer` 头为空，请求将被 **允许**。
    - 如果 `Referer` 是一个 IP 地址 (如 `http://127.0.0.1:3000`)，请求将被 **允许**。
    - 如果 `Referer` 是域名，但该域名在 **白名单** 中，请求将被 **允许**。

3.  **请求频率限制**:
    - 对于在白名单中的域名，每个域名每天的请求次数有上限（默认为 1000 次）。超过限制后，当天的后续请求将被拒绝。

如果您的正常访问被阻止，请登录管理后台，将您网站的域名添加到 Referer 白名单中。
