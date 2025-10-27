#!/bin/bash

# CDNProxy 自动扩容脚本
# 基于 CPU、内存、请求量等指标自动调整服务配置

HOST="http://localhost:8081"
LOG_FILE="/var/log/cdnproxy/auto_scale.log"
CONFIG_FILE="/etc/cdnproxy/config.env"

# 最大内存限制配置 (MB)
MAX_MEMORY_LIMIT_MB=2048  # 最大内存限制 2GB
MIN_MEMORY_LIMIT_MB=256   # 最小内存限制 256MB
MEMORY_SCALE_STEP_MB=256  # 每次扩容增加的内存

# 创建日志目录
mkdir -p "$(dirname "$LOG_FILE")"

# 日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# 获取当前指标
get_metrics() {
    local response=$(curl -s "$HOST/stats" 2>/dev/null)
    if [ $? -ne 0 ]; then
        log "ERROR: 无法连接到 CDNProxy 服务"
        return 1
    fi
    
    echo "$response"
}

# 检查是否需要扩容
check_scale_need() {
    local metrics="$1"
    
    # 解析指标
    local memory_mb=$(echo "$metrics" | jq -r '.system.memory_alloc_mb')
    local goroutines=$(echo "$metrics" | jq -r '.system.goroutines')
    local success_rate=$(echo "$metrics" | jq -r '.application.success_rate')
    local avg_response_time=$(echo "$metrics" | jq -r '.application.avg_response_time_ms')
    
    log "当前状态: 内存=${memory_mb}MB, Goroutines=${goroutines}, 成功率=${success_rate}%, 平均响应时间=${avg_response_time}ms"
    
    # 获取当前内存限制
    local current_memory_limit=$(grep "MAX_MEMORY_MB" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "512")
    
    # 扩容条件
    if [ "$memory_mb" -gt 400 ] || [ "$goroutines" -gt 500 ] || [ "$success_rate" -lt 95 ] || [ "$avg_response_time" -gt 2000 ]; then
        # 检查是否已达到最大内存限制
        if [ "$current_memory_limit" -ge "$MAX_MEMORY_LIMIT_MB" ]; then
            log "WARNING: 已达到最大内存限制 ${MAX_MEMORY_LIMIT_MB}MB，无法继续扩容"
            return 1  # 不能扩容
        fi
        return 0  # 需要扩容
    else
        return 1  # 不需要扩容
    fi
}

# 执行扩容
scale_up() {
    log "开始扩容操作..."
    
    # 1. 增加并发限制
    local current_concurrent=$(grep "MAX_CONCURRENT" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "50")
    local new_concurrent=$((current_concurrent + 20))
    
    # 2. 增加内存限制
    local current_memory=$(grep "MAX_MEMORY_MB" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "512")
    local new_memory=$((current_memory + MEMORY_SCALE_STEP_MB))
    
    # 检查是否超过最大内存限制
    if [ "$new_memory" -gt "$MAX_MEMORY_LIMIT_MB" ]; then
        new_memory=$MAX_MEMORY_LIMIT_MB
        log "WARNING: 内存限制已达到最大值 ${MAX_MEMORY_LIMIT_MB}MB"
    fi
    
    # 3. 更新配置
    if [ -f "$CONFIG_FILE" ]; then
        sed -i "s/MAX_CONCURRENT=.*/MAX_CONCURRENT=$new_concurrent/" "$CONFIG_FILE"
        sed -i "s/MAX_MEMORY_MB=.*/MAX_MEMORY_MB=$new_memory/" "$CONFIG_FILE"
    else
        echo "MAX_CONCURRENT=$new_concurrent" > "$CONFIG_FILE"
        echo "MAX_MEMORY_MB=$new_memory" >> "$CONFIG_FILE"
    fi
    
    # 4. 重启服务（如果使用 systemd）
    if systemctl is-active --quiet cdnproxy; then
        log "重启 CDNProxy 服务..."
        systemctl reload cdnproxy
    fi
    
    log "扩容完成: 并发限制=$new_concurrent, 内存限制=${new_memory}MB"
}

# 检查是否需要缩容
check_scale_down() {
    local metrics="$1"
    
    local memory_mb=$(echo "$metrics" | jq -r '.system.memory_alloc_mb')
    local goroutines=$(echo "$metrics" | jq -r '.system.goroutines')
    local success_rate=$(echo "$metrics" | jq -r '.application.success_rate')
    
    # 缩容条件：资源使用率低且稳定
    if [ "$memory_mb" -lt 100 ] && [ "$goroutines" -lt 100 ] && [ "$success_rate" -gt 99 ]; then
        return 0  # 可以缩容
    else
        return 1  # 不需要缩容
    fi
}

# 检查内存使用是否超过限制
check_memory_limit() {
    local metrics="$1"
    local memory_mb=$(echo "$metrics" | jq -r '.system.memory_alloc_mb')
    local current_memory_limit=$(grep "MAX_MEMORY_MB" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "512")
    
    # 如果内存使用超过当前限制的90%，触发紧急处理
    local threshold=$((current_memory_limit * 90 / 100))
    if [ "$memory_mb" -gt "$threshold" ]; then
        log "WARNING: 内存使用 ${memory_mb}MB 超过限制 ${current_memory_limit}MB 的90% (${threshold}MB)"
        return 0  # 需要紧急处理
    else
        return 1  # 正常
    fi
}

# 紧急内存处理
emergency_memory_handling() {
    log "执行紧急内存处理..."
    
    # 1. 清理缓存
    log "清理缓存目录..."
    rm -rf /var/cache/cdnproxy/* 2>/dev/null || true
    
    # 2. 强制垃圾回收（如果支持）
    curl -s -X POST "$HOST/admin/gc" > /dev/null 2>&1 || true
    
    # 3. 降低并发限制
    local current_concurrent=$(grep "MAX_CONCURRENT" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "50")
    local emergency_concurrent=$((current_concurrent / 2))
    if [ "$emergency_concurrent" -lt 10 ]; then
        emergency_concurrent=10
    fi
    
    if [ -f "$CONFIG_FILE" ]; then
        sed -i "s/MAX_CONCURRENT=.*/MAX_CONCURRENT=$emergency_concurrent/" "$CONFIG_FILE"
    else
        echo "MAX_CONCURRENT=$emergency_concurrent" > "$CONFIG_FILE"
    fi
    
    # 4. 重启服务
    if systemctl is-active --quiet cdnproxy; then
        log "重启 CDNProxy 服务以释放内存..."
        systemctl restart cdnproxy
    fi
    
    log "紧急内存处理完成: 并发限制降至 $emergency_concurrent"
}

# 执行缩容
scale_down() {
    log "开始缩容操作..."
    
    local current_concurrent=$(grep "MAX_CONCURRENT" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "50")
    local new_concurrent=$((current_concurrent - 10))
    
    # 确保最小并发数
    if [ "$new_concurrent" -lt 20 ]; then
        new_concurrent=20
    fi
    
    # 同时减少内存限制
    local current_memory=$(grep "MAX_MEMORY_MB" "$CONFIG_FILE" 2>/dev/null | cut -d'=' -f2 || echo "512")
    local new_memory=$((current_memory - MEMORY_SCALE_STEP_MB))
    
    # 确保最小内存限制
    if [ "$new_memory" -lt "$MIN_MEMORY_LIMIT_MB" ]; then
        new_memory=$MIN_MEMORY_LIMIT_MB
    fi
    
    if [ -f "$CONFIG_FILE" ]; then
        sed -i "s/MAX_CONCURRENT=.*/MAX_CONCURRENT=$new_concurrent/" "$CONFIG_FILE"
        sed -i "s/MAX_MEMORY_MB=.*/MAX_MEMORY_MB=$new_memory/" "$CONFIG_FILE"
    else
        echo "MAX_CONCURRENT=$new_concurrent" > "$CONFIG_FILE"
        echo "MAX_MEMORY_MB=$new_memory" >> "$CONFIG_FILE"
    fi
    
    if systemctl is-active --quiet cdnproxy; then
        log "重启 CDNProxy 服务..."
        systemctl reload cdnproxy
    fi
    
    log "缩容完成: 并发限制=$new_concurrent, 内存限制=${new_memory}MB"
}

# 验证配置
validate_config() {
    log "验证配置参数..."
    
    # 检查 jq 是否安装
    if ! command -v jq &> /dev/null; then
        log "ERROR: jq 未安装，请先安装 jq: apt-get install jq 或 yum install jq"
        exit 1
    fi
    
    # 检查 curl 是否安装
    if ! command -v curl &> /dev/null; then
        log "ERROR: curl 未安装，请先安装 curl"
        exit 1
    fi
    
    # 验证内存限制配置
    if [ "$MAX_MEMORY_LIMIT_MB" -lt "$MIN_MEMORY_LIMIT_MB" ]; then
        log "ERROR: 最大内存限制 ${MAX_MEMORY_LIMIT_MB}MB 不能小于最小内存限制 ${MIN_MEMORY_LIMIT_MB}MB"
        exit 1
    fi
    
    if [ "$MEMORY_SCALE_STEP_MB" -lt 1 ]; then
        log "ERROR: 内存扩容步长 ${MEMORY_SCALE_STEP_MB}MB 必须大于 0"
        exit 1
    fi
    
    log "配置验证通过"
    log "最大内存限制: ${MAX_MEMORY_LIMIT_MB}MB"
    log "最小内存限制: ${MIN_MEMORY_LIMIT_MB}MB"
    log "内存扩容步长: ${MEMORY_SCALE_STEP_MB}MB"
}

# 显示帮助信息
show_help() {
    cat << EOF
CDNProxy 自动扩容脚本

用法: $0 [选项]

选项:
    -h, --help          显示此帮助信息
    -c, --check         仅检查当前状态，不执行扩容操作
    -d, --daemon        以守护进程模式运行
    --max-memory MB     设置最大内存限制 (默认: $MAX_MEMORY_LIMIT_MB)
    --min-memory MB     设置最小内存限制 (默认: $MIN_MEMORY_LIMIT_MB)
    --step-memory MB    设置内存扩容步长 (默认: $MEMORY_SCALE_STEP_MB)

示例:
    $0                  # 正常运行
    $0 --check          # 仅检查状态
    $0 --daemon         # 守护进程模式
    $0 --max-memory 4096 --min-memory 512  # 自定义内存限制

EOF
}

# 主循环
main() {
    log "CDNProxy 自动扩容脚本启动"
    
    while true; do
        # 获取指标
        metrics=$(get_metrics)
        if [ $? -ne 0 ]; then
            log "获取指标失败，等待 30 秒后重试"
            sleep 30
            continue
        fi
        
        # 检查紧急内存处理
        if check_memory_limit "$metrics"; then
            emergency_memory_handling
        # 检查扩容需求
        elif check_scale_need "$metrics"; then
            scale_up
        elif check_scale_down "$metrics"; then
            scale_down
        else
            log "当前状态正常，无需调整"
        fi
        
        # 等待下次检查
        sleep 60
    done
}

# 信号处理
trap 'log "脚本被中断，退出"; exit 0' INT TERM

# 解析命令行参数
CHECK_ONLY=false
DAEMON_MODE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -c|--check)
            CHECK_ONLY=true
            shift
            ;;
        -d|--daemon)
            DAEMON_MODE=true
            shift
            ;;
        --max-memory)
            MAX_MEMORY_LIMIT_MB="$2"
            shift 2
            ;;
        --min-memory)
            MIN_MEMORY_LIMIT_MB="$2"
            shift 2
            ;;
        --step-memory)
            MEMORY_SCALE_STEP_MB="$2"
            shift 2
            ;;
        *)
            echo "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 验证配置
validate_config

# 如果只是检查状态
if [ "$CHECK_ONLY" = true ]; then
    log "执行状态检查..."
    metrics=$(get_metrics)
    if [ $? -eq 0 ]; then
        echo "当前状态:"
        echo "$metrics" | jq .
    else
        log "无法获取状态信息"
        exit 1
    fi
    exit 0
fi

# 启动主循环
main
