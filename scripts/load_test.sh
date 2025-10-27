#!/bin/bash

# CDNProxy 压力测试脚本
# 用于验证性能优化效果

echo "🚀 开始 CDNProxy 压力测试..."

# 测试参数
HOST="http://localhost:8081"
CONCURRENT=50
REQUESTS=1000

echo "📊 测试配置："
echo "  目标地址: $HOST"
echo "  并发数: $CONCURRENT"
echo "  总请求数: $REQUESTS"
echo ""

# 1. 健康检查测试
echo "1️⃣ 健康检查测试..."
curl -s "$HOST/healthz" | jq .
echo ""

# 2. 基础性能测试
echo "2️⃣ 基础性能测试 (ab)..."
ab -n 100 -c 10 "$HOST/healthz" | grep -E "(Requests per second|Time per request|Failed requests)"
echo ""

# 3. CDN 代理性能测试
echo "3️⃣ CDN 代理性能测试..."
ab -n 200 -c 20 "$HOST/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js" | grep -E "(Requests per second|Time per request|Failed requests)"
echo ""

# 4. 内存使用测试
echo "4️⃣ 内存使用测试..."
echo "测试前内存状态："
curl -s "$HOST/metrics" | grep memory_alloc_bytes

# 并发请求测试
echo "开始并发请求测试..."
for i in {1..5}; do
    echo "第 $i 轮测试..."
    ab -n 100 -c 20 "$HOST/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js" > /dev/null 2>&1
    sleep 2
    echo "当前内存使用："
    curl -s "$HOST/metrics" | grep memory_alloc_bytes
done

echo ""
echo "✅ 压力测试完成！"
