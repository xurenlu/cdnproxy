# 云函数部署指南

## 🚀 支持的云函数平台

- ✅ **腾讯云函数 (SCF)**
- ✅ **阿里云函数计算 (FC)**
- 🔄 **AWS Lambda** (计划中)
- 🔄 **Azure Functions** (计划中)

## 📋 部署前准备

### 1. 环境要求
- Go 1.19+
- 云函数平台账号
- 相应的CLI工具

### 2. 安装依赖
```bash
# 腾讯云
npm install -g serverless
npm install -g serverless-tencent-scf

# 阿里云
npm install -g @alicloud/fun
```

## 🎯 腾讯云函数部署

### 1. 配置腾讯云凭证
```bash
# 安装腾讯云CLI
pip install tencentcloud-sdk-python

# 配置凭证
tccli configure
```

### 2. 部署函数
```bash
cd deployments/tencent-cloud
serverless deploy
```

### 3. 环境变量配置
```bash
# 设置管理员密码
export ADMIN_PASSWORD=your_secure_password

# 部署
serverless deploy --env production
```

### 4. 测试部署
```bash
# 获取函数URL
serverless info

# 测试CDN代理
curl https://your-function-url.scf.tencentcs.com/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js

# 测试API代理
curl https://your-function-url.scf.tencentcs.com/api.openai.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## 🎯 阿里云函数计算部署

### 1. 配置阿里云凭证
```bash
# 安装阿里云CLI
fun config
```

### 2. 部署函数
```bash
cd deployments/aliyun-fc
fun deploy
```

### 3. 环境变量配置
```bash
# 设置环境变量
export ADMIN_PASSWORD=your_secure_password

# 部署
fun deploy --env production
```

### 4. 测试部署
```bash
# 获取函数URL
fun info

# 测试功能
curl https://your-function-url.fc.aliyuncs.com/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js
```

## ⚙️ 云函数配置说明

### 内存配置
```yaml
# 腾讯云
memorySize: 512  # 512MB

# 阿里云
MemorySize: 512  # 512MB
```

### 超时配置
```yaml
# 腾讯云
timeout: 30  # 30秒

# 阿里云
Timeout: 30  # 30秒
```

### 环境变量
```yaml
environment:
  SERVERLESS_PLATFORM: tencent  # 或 aliyun
  CACHE_DIR: /tmp/cache
  DATA_DIR: /tmp/data
  CACHE_TTL_SECONDS: 43200
  ADMIN_USERNAME: admin
  ADMIN_PASSWORD: ${env:ADMIN_PASSWORD}
```

## 🔧 云函数优化

### 1. 冷启动优化
```go
// 在init函数中预加载
func init() {
    // 预加载配置
    cfg := config.Load()
    
    // 预加载缓存
    cache := cache.NewDiskCache("/tmp/cache", 100*1024*1024)
    
    // 预加载处理器
    handler = proxy.NewHandler(cfg, cache, ...)
}
```

### 2. 内存优化
- 使用内存缓存替代磁盘缓存
- 限制缓存大小（100MB）
- 定期清理临时文件

### 3. 性能优化
- 复用HTTP客户端
- 使用连接池
- 异步处理非关键任务

## 📊 监控和日志

### 腾讯云监控
```bash
# 查看函数日志
serverless logs -f cdnproxy

# 查看函数指标
serverless metrics -f cdnproxy
```

### 阿里云监控
```bash
# 查看函数日志
fun logs -f cdnproxy

# 查看函数指标
fun metrics -f cdnproxy
```

## 🚨 注意事项

### 1. 限制和约束
- **执行时间限制**：最长30秒
- **内存限制**：最大3GB
- **请求大小限制**：6MB
- **响应大小限制**：6MB

### 2. 成本考虑
- **按调用次数计费**
- **按执行时间计费**
- **按内存使用计费**
- **免费额度**：每月100万次调用

### 3. 最佳实践
- 使用CDN加速静态资源
- 实现缓存策略
- 监控函数性能
- 设置告警机制

## 🔄 持续部署

### GitHub Actions 示例
```yaml
name: Deploy to Tencent Cloud
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.19
      - name: Build
        run: go build -o main cmd/serverless/main.go
      - name: Deploy to Tencent Cloud
        run: |
          cd deployments/tencent-cloud
          serverless deploy
        env:
          ADMIN_PASSWORD: ${{ secrets.ADMIN_PASSWORD }}
```

## 📈 性能对比

| 指标 | 传统部署 | 云函数部署 |
|------|----------|------------|
| 启动时间 | 30秒 | 1秒 |
| 内存使用 | 512MB | 100MB |
| 成本 | 固定 | 按需 |
| 扩展性 | 手动 | 自动 |
| 维护成本 | 高 | 低 |

## 🎯 适用场景

### 适合云函数部署
- 低并发场景（<1000 RPS）
- 突发流量
- 成本敏感
- 快速原型

### 不适合云函数部署
- 高并发场景（>1000 RPS）
- 长时间运行
- 大文件处理
- 复杂业务逻辑
