# 部署运维

CDNProxy 部署和运维指南。

## 📖 **内容**

### [Docker部署](docker.md)
使用 Docker 部署 CDNProxy

### [云函数部署](serverless.md)
部署到云函数平台

### [运维指南](operations.md)
日常运维和管理

## 🚀 **快速部署**

### Docker

```bash
docker run -d -p 8080:8080 cdnproxy/cdnproxy:latest
```

### Docker Compose

```bash
docker-compose up -d
```

### 云函数

请参考 [云函数部署](serverless.md)

## 📚 **相关文档**

- [快速开始](../01-getting-started/README.md)
- [故障排查](../06-troubleshooting/)
