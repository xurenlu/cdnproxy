# CDNProxy 使用说明

本文档将指导您如何使用本 CDNProxy 服务。

## 项目简介

CDNProxy 是一个轻量级的反向代理服务，旨在解决在特定网络环境下访问公共 CDN（如 cdn.jsdelivr.net, unpkg.com）困难的问题。它通过将 CDN 请求代理到您自己的服务器，并利用 Redis 进行响应缓存，来提高资源的访问速度和稳定性。

## 如何使用

您只需将目标 CDN 资源的 URL host 之前的部分，替换为本代理服务的地址即可。

### 示例 1: `cdn.jsdelivr.net`

**原始 URL:**
```
https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

**通过本代理访问:**
```
/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css
```

### 示例 2: `unpkg.com`

**原始 URL:**
```
https://unpkg.com/react@18/umd/react.development.js
```

**通过本代理访问:**
```
/unpkg.com/react@18/umd/react.development.js
```

## 访问控制说明

为了防止代理服务被滥用，服务实施了访问控制策略。如果您的请求被意外阻止，通常是因为您访问本服务的网站域名（即 Referer）不在白名单中。

如有需要，请联系服务管理员将您的网站域名加入白名单。
