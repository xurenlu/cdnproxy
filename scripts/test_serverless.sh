#!/bin/bash

# 云函数测试脚本
echo "🧪 测试 CDNProxy 云函数版本..."

# 测试腾讯云函数事件格式
echo "1️⃣ 测试腾讯云函数事件格式..."
tencent_event='{
  "httpMethod": "GET",
  "path": "/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js",
  "headers": {
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
    "Accept": "*/*"
  },
  "queryString": {},
  "body": "",
  "isBase64Encoded": false
}'

echo "腾讯云事件:"
echo "$tencent_event" | jq .

# 测试阿里云函数计算事件格式
echo ""
echo "2️⃣ 测试阿里云函数计算事件格式..."
aliyun_event='{
  "httpMethod": "GET",
  "path": "/cdn.jsdelivr.net/npm/vue@3/dist/vue.global.js",
  "headers": {
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
    "Accept": "*/*"
  },
  "queryParameters": {},
  "body": "",
  "isBase64Encoded": false
}'

echo "阿里云事件:"
echo "$aliyun_event" | jq .

# 测试API代理事件
echo ""
echo "3️⃣ 测试API代理事件..."
api_event='{
  "httpMethod": "POST",
  "path": "/api.openai.com/v1/chat/completions",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer sk-test-key"
  },
  "queryString": {},
  "body": "{\"model\":\"gpt-3.5-turbo\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}",
  "isBase64Encoded": false
}'

echo "API代理事件:"
echo "$api_event" | jq .

echo ""
echo "✅ 云函数事件格式测试完成！"
echo ""
echo "📋 部署说明："
echo "1. 腾讯云函数: cd deployments/tencent-cloud && serverless deploy"
echo "2. 阿里云函数计算: cd deployments/aliyun-fc && fun deploy"
echo "3. 设置环境变量: export ADMIN_PASSWORD=your_password"
