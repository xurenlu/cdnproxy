# CDNProxy 运维指南

## 📋 目录
- [部署指南](#部署指南)
- [监控配置](#监控配置)
- [性能调优](#性能调优)
- [故障排查](#故障排查)
- [维护操作](#维护操作)

## 🚀 部署指南

### 1. 基础部署

```bash
# 1. 克隆项目
git clone https://github.com/xurenlu/cdnproxy.git
cd cdnproxy

# 2. 编译
go build -o cdnproxy .

# 3. 配置环境变量
export PORT=8080
export CACHE_DIR=/var/cache/cdnproxy
export DATA_DIR=/var/lib/cdnproxy

# 4. 启动服务
./cdnproxy
```

### 2. Docker 部署

```bash
# 构建镜像
docker build -t cdnproxy:latest .

# 运行容器
docker run -d \
  --name cdnproxy \
  -p 8080:8080 \
  -v /var/cache/cdnproxy:/var/cache/cdnproxy \
  -v /var/lib/cdnproxy:/var/lib/cdnproxy \
  cdnproxy:latest
```

### 3. Docker Compose 部署

```bash
# 启动服务
docker-compose up -d

# 启动监控栈
cd monitoring
docker-compose -f docker-compose.monitoring.yml up -d
```

## 📊 监控配置

### 1. 基础监控

访问以下端点获取监控数据：

- **健康检查**: `http://localhost:8080/healthz`
- **性能指标**: `http://localhost:8080/metrics` (Prometheus 格式)
- **详细统计**: `http://localhost:8080/stats` (JSON 格式)

### 2. Grafana 监控

1. 启动监控栈：
```bash
cd monitoring
docker-compose -f docker-compose.monitoring.yml up -d
```

2. 访问 Grafana：
   - URL: http://localhost:3000
   - 用户名: admin
   - 密码: admin123

3. 导入仪表板：
   - 使用 `monitoring/grafana-dashboard.json` 文件

### 3. 告警配置

#### 关键指标阈值：
- 内存使用 > 80%
- Goroutine 数量 > 1000
- 成功率 < 95%
- 平均响应时间 > 2秒
- 缓存命中率 < 70%

## ⚡ 性能调优

### 1. 系统级优化

```bash
# 增加文件描述符限制
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# 优化网络参数
echo "net.core.somaxconn = 65535" >> /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog = 65535" >> /etc/sysctl.conf
sysctl -p
```

### 2. 应用级优化

#### 环境变量配置：
```bash
# 并发控制
export MAX_CONCURRENT=100
export MAX_WEBSOCKET_CONNECTIONS=20
export MAX_WEBP_CONCURRENT=10

# 内存控制
export MAX_MEMORY_MB=1024
export LARGE_FILE_THRESHOLD_MB=2

# 缓存配置
export CACHE_MAX_SIZE_MB=500
export CACHE_CLEANUP_INTERVAL=1h
```

### 3. 自动扩容

```bash
# 启动自动扩容脚本
./scripts/auto_scale.sh &

# 或使用 systemd 服务
sudo cp scripts/cdnproxy-auto-scale.service /etc/systemd/system/
sudo systemctl enable cdnproxy-auto-scale
sudo systemctl start cdnproxy-auto-scale
```

## 🔧 故障排查

### 1. 常见问题

#### 服务无法启动
```bash
# 检查端口占用
netstat -tlnp | grep :8080

# 检查权限
ls -la /var/cache/cdnproxy
ls -la /var/lib/cdnproxy

# 查看日志
journalctl -u cdnproxy -f
```

#### 内存使用过高
```bash
# 查看内存使用
curl -s http://localhost:8080/metrics | grep memory

# 检查缓存大小
du -sh /var/cache/cdnproxy

# 清理缓存
rm -rf /var/cache/cdnproxy/*
```

#### 响应缓慢
```bash
# 查看慢请求日志
grep "SLOW REQUEST" /var/log/cdnproxy/cdnproxy.log

# 检查系统负载
top
iostat -x 1

# 检查网络延迟
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/healthz
```

### 2. 日志分析

#### 关键日志模式：
```bash
# 慢请求
grep "SLOW REQUEST" /var/log/cdnproxy/cdnproxy.log

# 错误请求
grep "ERROR" /var/log/cdnproxy/cdnproxy.log

# 缓存命中率
grep "cache" /var/log/cdnproxy/cdnproxy.log | tail -100
```

### 3. 性能分析

```bash
# 运行性能测试
./scripts/performance_test.sh

# 压力测试
./scripts/load_test.sh

# 内存分析
go tool pprof http://localhost:8080/debug/pprof/heap
```

## 🛠 维护操作

### 1. 定期维护

#### 每日检查：
```bash
# 检查服务状态
systemctl status cdnproxy

# 检查磁盘空间
df -h /var/cache/cdnproxy

# 检查内存使用
curl -s http://localhost:8080/healthz | jq .
```

#### 每周维护：
```bash
# 清理过期缓存
curl -X POST http://localhost:8080/admin/cleanup

# 备份配置
cp -r /var/lib/cdnproxy /backup/cdnproxy-$(date +%Y%m%d)

# 更新服务
git pull && go build && systemctl restart cdnproxy
```

### 2. 缓存管理

```bash
# 查看缓存统计
curl -s http://localhost:8080/stats | jq .application

# 清理所有缓存
rm -rf /var/cache/cdnproxy/*

# 设置缓存大小限制
export CACHE_MAX_SIZE_MB=1000
```

### 3. 配置更新

```bash
# 更新白名单
curl -X POST http://localhost:8080/admin/whitelist \
  -H "Content-Type: application/json" \
  -d '{"domains": ["example.com", "test.com"]}'

# 更新配置
curl -X POST http://localhost:8080/admin/config \
  -H "Content-Type: application/json" \
  -d '{"maxConcurrent": 200}'
```

## 📈 性能基准

### 1. 预期性能指标

| 指标 | 目标值 | 警告阈值 | 危险阈值 |
|------|--------|----------|----------|
| 请求处理能力 | > 1000 RPS | < 500 RPS | < 200 RPS |
| 平均响应时间 | < 100ms | > 500ms | > 2000ms |
| 内存使用 | < 512MB | > 800MB | > 1GB |
| 缓存命中率 | > 80% | < 60% | < 40% |
| 成功率 | > 99% | < 95% | < 90% |

### 2. 容量规划

| 并发用户 | 推荐配置 | 内存需求 | 存储需求 |
|----------|----------|----------|----------|
| 100 | 1核2G | 256MB | 10GB |
| 1000 | 2核4G | 512MB | 50GB |
| 10000 | 4核8G | 1GB | 200GB |
| 100000 | 8核16G | 2GB | 1TB |

## 🚨 应急响应

### 1. 服务不可用

```bash
# 1. 检查服务状态
systemctl status cdnproxy

# 2. 重启服务
systemctl restart cdnproxy

# 3. 检查资源使用
top
df -h
free -h

# 4. 查看错误日志
journalctl -u cdnproxy --since "10 minutes ago"
```

### 2. 性能严重下降

```bash
# 1. 降低并发限制
export MAX_CONCURRENT=50

# 2. 清理缓存
rm -rf /var/cache/cdnproxy/*

# 3. 重启服务
systemctl restart cdnproxy

# 4. 监控恢复情况
watch -n 5 'curl -s http://localhost:8080/healthz | jq .'
```

### 3. 内存泄漏

```bash
# 1. 生成内存快照
go tool pprof http://localhost:8080/debug/pprof/heap

# 2. 分析内存使用
(pprof) top10
(pprof) list main.main

# 3. 重启服务
systemctl restart cdnproxy
```

## 📞 联系支持

- **项目地址**: https://github.com/xurenlu/cdnproxy
- **问题报告**: https://github.com/xurenlu/cdnproxy/issues
- **文档**: https://github.com/xurenlu/cdnproxy/docs

---

**注意**: 本指南基于 v2.1.1 版本编写，请根据实际部署版本调整相关配置。
