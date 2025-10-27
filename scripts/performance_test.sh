#!/bin/bash

# CDNProxy 性能测试套件
# 全面测试各种场景下的性能表现

HOST="http://localhost:8081"
RESULTS_DIR="./test_results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# 创建结果目录
mkdir -p "$RESULTS_DIR"

echo "🚀 CDNProxy 性能测试套件启动"
echo "测试时间: $(date)"
echo "目标地址: $HOST"
echo "结果目录: $RESULTS_DIR"
echo ""

# 测试函数
run_test() {
    local test_name="$1"
    local url="$2"
    local concurrent="$3"
    local requests="$4"
    local output_file="$RESULTS_DIR/${test_name}_${TIMESTAMP}.txt"
    
    echo "🧪 执行测试: $test_name"
    echo "   URL: $url"
    echo "   并发: $concurrent, 请求数: $requests"
    
    # 使用 ab 进行压力测试
    ab -n "$requests" -c "$concurrent" -g "$RESULTS_DIR/${test_name}_${TIMESTAMP}.gnuplot" "$url" > "$output_file" 2>&1
    
    # 提取关键指标
    local rps=$(grep "Requests per second" "$output_file" | awk '{print $4}')
    local avg_time=$(grep "Time per request" "$output_file" | head -1 | awk '{print $4}')
    local failed=$(grep "Failed requests" "$output_file" | awk '{print $3}')
    
    echo "   结果: RPS=$rps, 平均时间=${avg_time}ms, 失败=$failed"
    echo ""
}

# 1. 健康检查性能测试
echo "📊 1. 健康检查性能测试"
run_test "health_check" "$HOST/healthz" 10 100

# 2. 小文件 CDN 代理测试
echo "📊 2. 小文件 CDN 代理测试"
run_test "cdn_small" "$HOST/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js" 20 200

# 3. 大文件 CDN 代理测试
echo "📊 3. 大文件 CDN 代理测试"
run_test "cdn_large" "$HOST/cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" 10 50

# 4. 缓存命中测试
echo "📊 4. 缓存命中测试"
# 先请求一次建立缓存
curl -s "$HOST/cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js" > /dev/null
# 然后测试缓存命中性能
run_test "cache_hit" "$HOST/cdn.jsdelivr.net/npm/jquery@3.6.0/dist/jquery.min.js" 30 300

# 5. 高并发测试
echo "📊 5. 高并发测试"
run_test "high_concurrency" "$HOST/healthz" 50 500

# 6. 长时间运行测试
echo "📊 6. 长时间运行测试 (5分钟)"
echo "开始长时间运行测试..."
start_time=$(date +%s)
end_time=$((start_time + 300))  # 5分钟

while [ $(date +%s) -lt $end_time ]; do
    ab -n 50 -c 10 "$HOST/healthz" > /dev/null 2>&1
    sleep 10
done

echo "长时间运行测试完成"

# 7. 内存泄漏测试
echo "📊 7. 内存泄漏测试"
echo "测试前内存状态:"
curl -s "$HOST/metrics" | grep memory_alloc_bytes

echo "执行 1000 次请求..."
for i in {1..10}; do
    ab -n 100 -c 20 "$HOST/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js" > /dev/null 2>&1
    sleep 1
done

echo "测试后内存状态:"
curl -s "$HOST/metrics" | grep memory_alloc_bytes

# 8. 错误处理测试
echo "📊 8. 错误处理测试"
run_test "error_handling" "$HOST/nonexistent/path" 5 50

# 生成测试报告
echo "📋 生成测试报告..."
report_file="$RESULTS_DIR/performance_report_${TIMESTAMP}.md"

cat > "$report_file" << EOF
# CDNProxy 性能测试报告

**测试时间**: $(date)
**目标地址**: $HOST
**测试环境**: $(uname -a)

## 测试结果摘要

EOF

# 添加各测试结果
for file in "$RESULTS_DIR"/*_${TIMESTAMP}.txt; do
    if [ -f "$file" ]; then
        test_name=$(basename "$file" | sed "s/_${TIMESTAMP}.txt//")
        echo "### $test_name" >> "$report_file"
        echo '```' >> "$report_file"
        grep -E "(Requests per second|Time per request|Failed requests|Connection Times)" "$file" >> "$report_file"
        echo '```' >> "$report_file"
        echo "" >> "$report_file"
    fi
done

# 添加系统指标
echo "### 系统指标" >> "$report_file"
echo '```' >> "$report_file"
curl -s "$HOST/stats" | jq . >> "$report_file"
echo '```' >> "$report_file"

echo "✅ 性能测试完成！"
echo "📊 测试报告: $report_file"
echo "📁 详细结果: $RESULTS_DIR"
