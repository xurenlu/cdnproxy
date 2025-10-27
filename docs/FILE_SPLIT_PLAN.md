# 文件拆分计划

## 📊 **大文件分析**

### 需要拆分的文件

| 文件 | 行数 | 状态 | 拆分建议 |
|------|------|------|----------|
| internal/proxy/handler.go | 820 | 🔴 需要拆分 | 拆分4个文件 |
| internal/ai/ai_optimizer.go | 487 | 🟡 可优化 | 保留或拆分 |
| internal/security/enterprise_security.go | 463 | 🟡 可优化 | 保留或拆分 |
| internal/admin/server.go | 444 | 🟡 可优化 | 拆分2个文件 |

## 🎯 **拆分方案**

### 1. handler.go (820行) → 拆分为4个文件

#### 1.1 cdn_handler.go (~300行)
**包含内容：**
- CDN代理核心逻辑
- 缓存处理和流式传输
- 大文件处理
- 主要 ServeHTTP 逻辑

**功能：**
- `ServeHTTP`
- `proxyNoCache`
- `buildUpstreamURL`
- 缓存相关逻辑

#### 1.2 webp_converter.go (~200行)
**包含内容：**
- WebP转换逻辑
- 图片格式处理
- User-Agent缓存
- 浏览器版本提取

**功能：**
- `shouldConvertToWebP`
- `convertToWebP`
- `extractFirefoxVersion`
- `extractSafariVersion`
- `storeUACache`

#### 1.3 access_control.go (~150行)
**包含内容：**
- 访问控制逻辑
- Whitelist处理
- Referer验证
- User-Agent验证

**功能：**
- `isAccessAllowed`
- Whitelist相关函数
- Referer验证函数

#### 1.4 compression.go (~150行)
**包含内容：**
- 压缩处理逻辑
- gzip压缩
- Accept-Encoding处理

**功能：**
- `compressBody`
- 压缩相关辅助函数

### 2. ai_optimizer.go (487行) → 保留或拆分

**选项1：保留**（如果功能完整且模块化良好）
**选项2：拆分**为以下文件：
- `ai_cache.go` - 智能缓存
- `ai_scheduler.go` - 流量调度
- `ai_anomaly.go` - 异常检测

### 3. enterprise_security.go (463行) → 保留或拆分

**选项1：保留**（如果功能完整且模块化良好）
**选项2：拆分**为以下文件：
- `waf.go` - WAF防护
- `ddos.go` - DDoS防护
- `auth.go` - 认证授权

### 4. server.go (444行) → 拆分为2个文件

#### 4.1 admin_server.go (~300行)
**包含内容：**
- HTTP处理器
- 路由管理
- 会话管理

#### 4.2 admin_templates.go (~150行)
**包含内容：**
- HTML模板
- 页面渲染
- UI相关代码

## ✅ **立即执行计划**

### 优先级1：拆分 handler.go
理由：最优先处理，最大且最核心

### 优先级2：拆分 server.go  
理由：管理面板相对独立，易于拆分

### 优先级3：评估 ai_optimizer.go 和 enterprise_security.go
理由：根据实际情况决定是否拆分

## 🚀 **实施步骤**

1. 创建新文件
2. 移动相关函数
3. 更新导入
4. 测试编译
5. 运行测试
6. 提交代码

## 📝 **注意事项**

- 保持向后兼容
- 不影响现有功能
- 保持代码可读性
- 更新相关文档
