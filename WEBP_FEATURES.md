# WebP 自动转换功能

## 功能概述

CDN代理现在支持自动将图片转换为WebP格式，以提供更好的压缩率和更快的加载速度。

## 支持的浏览器

- **Chrome/Chromium** 23+ (2012年9月)
- **Firefox** 65+ (2019年1月)
- **Safari** 14+ (2020年9月)
- **Edge** 18+ (2018年10月)
- **Opera** 12+

## 转换条件

1. **图片格式**：支持 JPEG、PNG、GIF 转 WebP
2. **浏览器支持**：主要基于 User-Agent 判断，Accept 头作为补充
3. **压缩效果**：只有WebP版本更小才使用转换后的图片

## 技术细节

### 检测逻辑
```go
// 主要基于User-Agent判断（更可靠）
if strings.Contains(userAgent, "chrome") && !strings.Contains(userAgent, "edg") {
    return true
}

// 精确的版本检测
if version := extractFirefoxVersion(userAgent); version >= 65 {
    return true
}

// Accept头作为补充（适用于API客户端）
if strings.Contains(acceptHeader, "image/webp") {
    return true
}
```

### 转换设置
```go
options := &webp.Options{
    Lossless: false, // 有损压缩
    Quality:  80,    // 质量80%
}
```

## 性能优化

1. **智能转换**：只有WebP版本更小才使用
2. **缓存策略**：转换后的WebP图片会被缓存
3. **版本检测**：精确的浏览器版本检测避免不必要的转换

## 使用示例

### 请求头示例
```
Accept: image/webp,image/apng,image/*,*/*;q=0.8
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36
```

### 响应头示例
```
Content-Type: image/webp
Content-Encoding: gzip
Cache-Control: public, max-age=86400
Vary: Accept-Encoding
```

## 压缩效果

根据测试，WebP格式通常能提供：
- **JPEG转WebP**：20-35% 文件大小减少
- **PNG转WebP**：25-50% 文件大小减少
- **GIF转WebP**：显著减少文件大小

## 注意事项

1. **CPU使用**：图片转换会增加服务器CPU使用
2. **内存使用**：大图片转换需要更多内存
3. **缓存策略**：建议为转换后的图片设置合适的缓存时间

## 配置建议

对于高流量网站，建议：
1. 启用Redis缓存
2. 设置合适的缓存TTL
3. 监控CPU和内存使用情况
4. 考虑异步转换大图片
