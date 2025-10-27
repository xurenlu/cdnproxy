// Package proxy WebP图片转换器
// 作者: rocky<m@some.im>

package proxy

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"strings"
	"sync"

	"github.com/chai2010/webp"
)

// WebPConverter WebP转换器
type WebPConverter struct {
	uaCache      sync.Map  // User-Agent解析缓存
	uaCacheSize  int64     // 缓存大小计数器
	uaCacheMutex sync.Mutex // 缓存大小保护锁
	semaphore    chan struct{} // WebP转换并发限制
}

// NewWebPConverter 创建WebP转换器
func NewWebPConverter() *WebPConverter {
	return &WebPConverter{
		semaphore: make(chan struct{}, 5), // 最多5个WebP转换并发（CPU密集）
	}
}

// storeUACache 存储User-Agent缓存
func (wc *WebPConverter) storeUACache(key string, value bool) {
	const maxCacheSize = 10000 // 最多缓存10000个User-Agent

	wc.uaCacheMutex.Lock()
	defer wc.uaCacheMutex.Unlock()

	// 如果缓存已满，清空缓存
	if wc.uaCacheSize >= maxCacheSize {
		log.Printf("UA cache size limit reached (%d), clearing cache", maxCacheSize)
		wc.uaCache = sync.Map{}
		wc.uaCacheSize = 0
	}

	wc.uaCache.Store(key, value)
	wc.uaCacheSize++
}

// ShouldConvertToWebP 判断是否应该转换为WebP格式
func (wc *WebPConverter) ShouldConvertToWebP(contentType, userAgent, acceptHeader string) bool {
	// 只处理图片类型
	if !strings.HasPrefix(contentType, "image/") {
		return false
	}

	// 检查User-Agent缓存
	cacheKey := userAgent + "|" + acceptHeader
	if cached, ok := wc.uaCache.Load(cacheKey); ok {
		return cached.(bool)
	}

	// 检查User-Agent是否支持WebP（主要判断依据）
	userAgent = strings.ToLower(userAgent)

	// Chrome/Chromium 23+ 支持WebP
	if strings.Contains(userAgent, "chrome") && !strings.Contains(userAgent, "edg") {
		result := true
		wc.storeUACache(cacheKey, result)
		return result
	}

	// Firefox 65+ 支持WebP (2019年1月发布)
	if strings.Contains(userAgent, "firefox") {
		// 提取Firefox版本号进行更精确的判断
		if version := wc.extractFirefoxVersion(userAgent); version >= 65 {
			result := true
			wc.storeUACache(cacheKey, result)
			return result
		}
	}

	// Safari 14+ 支持WebP (2020年9月发布)
	if strings.Contains(userAgent, "safari") && !strings.Contains(userAgent, "chrome") {
		// 提取Safari版本号
		if version := wc.extractSafariVersion(userAgent); version >= 14 {
			result := true
			wc.storeUACache(cacheKey, result)
			return result
		}
	}

	// Edge 18+ 支持WebP (2018年10月发布)
	if strings.Contains(userAgent, "edg") {
		result := true
		wc.storeUACache(cacheKey, result)
		return result
	}

	// Opera 12+ 支持WebP
	if strings.Contains(userAgent, "opr") || strings.Contains(userAgent, "opera") {
		result := true
		wc.storeUACache(cacheKey, result)
		return result
	}

	// 如果User-Agent不支持，但Accept头明确支持WebP，也可以转换
	// 这适用于一些API客户端或特殊工具
	if strings.Contains(acceptHeader, "image/webp") {
		result := true
		wc.storeUACache(cacheKey, result)
		return result
	}

	result := false
	wc.storeUACache(cacheKey, result)
	return result
}

// extractFirefoxVersion 提取Firefox版本号
func (wc *WebPConverter) extractFirefoxVersion(userAgent string) int {
	// 查找 "Firefox/" 后的版本号
	start := strings.Index(userAgent, "firefox/")
	if start == -1 {
		return 0
	}
	start += 8 // "firefox/".length

	// 找到版本号结束位置
	end := start
	for end < len(userAgent) && (userAgent[end] >= '0' && userAgent[end] <= '9' || userAgent[end] == '.') {
		end++
	}

	// 解析主版本号
	versionStr := userAgent[start:end]
	if dotIndex := strings.Index(versionStr, "."); dotIndex != -1 {
		versionStr = versionStr[:dotIndex]
	}

	// 转换为整数
	var version int
	fmt.Sscanf(versionStr, "%d", &version)
	return version
}

// extractSafariVersion 提取Safari版本号
func (wc *WebPConverter) extractSafariVersion(userAgent string) int {
	// 查找 "Version/" 后的版本号
	start := strings.Index(userAgent, "version/")
	if start == -1 {
		return 0
	}
	start += 8 // "version/".length

	// 找到版本号结束位置
	end := start
	for end < len(userAgent) && (userAgent[end] >= '0' && userAgent[end] <= '9' || userAgent[end] == '.') {
		end++
	}

	// 解析主版本号
	versionStr := userAgent[start:end]
	if dotIndex := strings.Index(versionStr, "."); dotIndex != -1 {
		versionStr = versionStr[:dotIndex]
	}

	// 转换为整数
	var version int
	fmt.Sscanf(versionStr, "%d", &version)
	return version
}

// ConvertToWebP 将图片转换为WebP格式
func (wc *WebPConverter) ConvertToWebP(body []byte, contentType string) ([]byte, error) {
	// 获取并发许可
	wc.semaphore <- struct{}{}
	defer func() { <-wc.semaphore }()

	// 只处理支持的图片格式
	if !strings.Contains(contentType, "image/jpeg") &&
		!strings.Contains(contentType, "image/png") &&
		!strings.Contains(contentType, "image/gif") {
		return nil, fmt.Errorf("unsupported image type: %s", contentType)
	}

	// 使用defer recover防止image.Decode panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in image decode: %v", r)
		}
	}()

	// 解码图片
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	// 检查图片尺寸，防止内存占用过大
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	const maxPixels = 4096 * 4096 // 最大16M像素
	if width*height > maxPixels {
		return nil, fmt.Errorf("image too large: %dx%d pixels", width, height)
	}

	// 转换为WebP格式
	var buf bytes.Buffer

	// 设置WebP编码选项
	options := &webp.Options{
		Lossless: false, // 使用有损压缩以获得更好的压缩率
		Quality:  80,    // 质量设置为80，平衡文件大小和图片质量
	}

	// 编码为WebP
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %v", err)
	}

	webpData := buf.Bytes()

	// 只有WebP版本确实更小才返回WebP
	if len(webpData) < len(body) {
		return webpData, nil
	}

	// 如果WebP版本更大，返回原始图片
	return body, nil
}
