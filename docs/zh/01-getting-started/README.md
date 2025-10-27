# 快速开始

欢迎使用 CDNProxy！

## 📖 **什么是 CDNProxy？**

CDNProxy 是一个高性能的 CDN 和 API 代理服务，具有以下特性：

- ✅ **CDN 代理**: 加速全球 CDN 资源访问
- ✅ **API 代理**: 支持 AI API 代理，包括 OpenAI、Claude、Gemini
- ✅ **住宅IP代理**: 支持多个住宅IP提供者
- ✅ **智能优化**: AI 驱动的智能缓存和路由
- ✅ **性能监控**: 完整的性能指标和监控
- ✅ **成本优化**: 智能成本控制和优化

## 🚀 **快速安装**

### 方式一：直接运行

```bash
# 下载最新版本
wget https://github.com/xurenlu/cdnproxy/releases/latest/download/cdnproxy

# 添加执行权限
chmod +x cdnproxy

# 运行
./cdnproxy
```

### 方式二：Docker

```bash
# 拉取镜像
docker pull cdnproxy/cdnproxy:latest

# 运行容器
docker run -d -p 8080:8080 cdnproxy/cdnproxy:latest
```

### 方式三：源码编译

```bash
# 克隆代码
git clone https://github.com/xurenlu/cdnproxy.git
cd cdnproxy

# 编译
go build -o cdnproxy

# 运行
./cdnproxy
```

## ⚙️ **配置**

### 环境变量

```bash
# 端口配置
export PORT=8080

# 数据目录
export DATA_DIR=./data

# Admin 配置
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=your_password

# 住宅IP代理配置（可选）
export BRIGHT_DATA_API_KEY=your_api_key
export BRIGHT_DATA_USERNAME=your_username
export BRIGHT_DATA_PASSWORD=your_password
```

### 配置文件

创建 `config.yml`:

```yaml
port: 8080
data_dir: ./data
admin:
  username: admin
  password: your_password
residential_proxy:
  providers:
    bright_data:
      enabled: true
      api_key: your_api_key
```

## 🎯 **使用示例**

### CDN 代理

```bash
# 代理 jsDelivr CDN
curl "https://cdnproxy.shifen.de/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js"
```

### API 代理

```bash
# OpenAI API
curl "https://cdnproxy.shifen.de/api.openai.com/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_KEY" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello!"}]}'
```

## 📚 **下一步**

- 查看 [用户指南](../02-user-guide/)
- 了解 [API参考](../03-api-reference/)
- 查看 [部署指南](../04-deployment/)

## 💡 **需要帮助？**

- 查看 [故障排查](../06-troubleshooting/)
- 提交 Issue: https://github.com/xurenlu/cdnproxy/issues
- 查看 [FAQ](../06-troubleshooting/README.md)