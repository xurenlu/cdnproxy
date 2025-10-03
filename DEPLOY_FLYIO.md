# Fly.io 快速部署指南

5 分钟将你的 CDN 代理部署到香港节点！

## 前置要求

- 一个 Fly.io 账号（免费）
- 本地已安装 flyctl
- 项目代码

## 一、安装 flyctl

### macOS
```bash
brew install flyctl
```

### Linux/WSL
```bash
curl -L https://fly.io/install.sh | sh
```

### 验证安装
```bash
flyctl version
```

## 二、登录 Fly.io

```bash
flyctl auth login
```

会打开浏览器进行登录，或注册新账号（免费）。

## 三、创建应用

在项目根目录执行：

```bash
# 创建应用（已有 fly.toml 会读取配置）
flyctl launch

# 交互式问题：
# - Choose an app name: cdnproxy-你的名字（可选，留空自动生成）
# - Choose a region: hkg (Hong Kong)
# - Would you like to set up a PostgreSQL database? No
# - Would you like to set up an Upstash Redis database? Yes ✅ （推荐）
# - Would you like to deploy now? No （稍后部署）
```

如果选择了 Redis，Fly.io 会自动创建 Redis 实例并设置 `REDIS_URL` 环境变量。

## 四、配置密钥

设置管理员密码（必须）：

```bash
flyctl secrets set ADMIN_PASSWORD="你的强密码"
```

如果没有自动创建 Redis，手动设置：

```bash
flyctl secrets set REDIS_URL="redis://..."
```

## 五、部署

```bash
flyctl deploy
```

首次部署大约需要 2-5 分钟。

## 六、验证部署

```bash
# 查看应用状态
flyctl status

# 查看日志
flyctl logs

# 打开应用
flyctl open

# 查看应用信息
flyctl info
```

访问 `https://你的应用名.fly.dev/healthz` 应该返回 `ok`。

## 七、测试性能

```bash
# 测试代理下载（替换为你的域名）
time curl -o /dev/null "https://你的应用名.fly.dev/cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js"
```

应该看到速度明显提升！

## 八、自定义域名（可选）

```bash
# 添加证书和域名
flyctl certs create 你的域名.com

# 按提示配置 DNS：
# - 添加 CNAME 记录指向 你的应用名.fly.dev
# 或
# - 添加 A/AAAA 记录指向 Fly.io IP
```

## 高级配置

### 扩展到多区域

```bash
# 添加新加坡区域
flyctl regions add sin

# 添加东京区域
flyctl regions add nrt

# 查看当前区域
flyctl regions list
```

### 增加实例数量

```bash
# 扩展到 2 个实例
flyctl scale count 2

# 查看当前扩展
flyctl scale show
```

### 升级 VM 规格

```bash
# 升级到 1GB 内存
flyctl scale memory 1024

# 升级到 2 个 CPU
flyctl scale vm shared-cpu-2x
```

### 查看 Redis 信息

```bash
flyctl redis list
flyctl redis status 你的redis名
```

## 监控和维护

### 查看实时日志

```bash
flyctl logs -a cdnproxy
```

### 查看指标

```bash
flyctl dashboard
```

在浏览器中查看：
- 请求数
- 响应时间
- 错误率
- CPU/内存使用

### SSH 到容器

```bash
flyctl ssh console
```

### 重启应用

```bash
flyctl apps restart
```

## 更新部署

修改代码后，重新部署：

```bash
git add .
git commit -m "更新功能"
flyctl deploy
```

## 回滚

```bash
# 查看历史版本
flyctl releases

# 回滚到上一个版本
flyctl releases rollback
```

## 成本估算

Fly.io 免费额度（Hobby Plan）：
- ✅ 最多 3 个 shared-cpu-1x (256MB) VMs
- ✅ 160GB 带宽/月
- ✅ 3GB 存储

**估算使用量**：
- 单实例香港部署：**完全免费** ✅
- 双区域部署（香港+新加坡）：**完全免费** ✅
- 三区域部署：**完全免费** ✅
- 超出流量：$0.02/GB

**对于中等流量 CDN 代理，完全可以免费使用！**

## 故障排查

### 应用无法访问

```bash
# 检查健康检查
flyctl status

# 查看日志
flyctl logs

# 检查 Redis 连接
flyctl ssh console
nc -zv redis.hostname 6379
```

### Redis 连接失败

```bash
# 查看 secrets
flyctl secrets list

# 重新设置 REDIS_URL
flyctl secrets set REDIS_URL="redis://..."
```

### 性能仍然慢

1. 确认部署在正确的区域：
```bash
flyctl regions list
```

2. 检查是否启用了多区域但没有足够实例：
```bash
flyctl scale count 3  # 至少等于区域数量
```

3. 查看日志是否有上游 CDN 限速：
```bash
flyctl logs | grep "upstream"
```

## 完整命令清单

```bash
# 安装
brew install flyctl

# 登录
flyctl auth login

# 创建应用
flyctl launch --region hkg

# 设置密钥
flyctl secrets set ADMIN_PASSWORD="xxx"

# 部署
flyctl deploy

# 查看状态
flyctl status
flyctl logs
flyctl info

# 扩展
flyctl regions add sin
flyctl scale count 2

# 自定义域名
flyctl certs create domain.com

# 更新
git commit && flyctl deploy

# 回滚
flyctl releases rollback
```

## 迁移现有 Railway 应用

如果你已经在 Railway 运行：

1. 备份 Redis 数据（如果需要）
2. 部署到 Fly.io（按上述步骤）
3. 测试新部署正常工作
4. 更新 DNS 指向 Fly.io
5. 监控 24 小时
6. 停止 Railway 应用

## 下一步

- [ ] 配置自定义域名
- [ ] 设置监控告警
- [ ] 添加更多区域（如需要）
- [ ] 配置 CDN（Cloudflare）在 Fly.io 前面（可选）

## 获取帮助

- 官方文档：https://fly.io/docs/
- 社区论坛：https://community.fly.io/
- Discord：https://fly.io/discord

---

**预期效果**：部署到香港节点后，中国用户的下载速度应该从 10-15 分钟降低到 3-5 秒，提升 **100-200 倍**！🚀

