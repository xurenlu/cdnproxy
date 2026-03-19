package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cdnproxy/internal/cache"
	"cdnproxy/internal/config"
)

// CacheHandler 缓存处理器
type CacheHandler struct {
	cfg   config.Config
	cache cache.CacheInterface
}

// NewCacheHandler 创建缓存处理器
func NewCacheHandler(cfg config.Config, cacheStore cache.CacheInterface) *CacheHandler {
	return &CacheHandler{
		cfg:   cfg,
		cache: cacheStore,
	}
}

// ServeCached 如果缓存命中，直接返回缓存内容
// 返回: (是否命中, 错误)
func (ch *CacheHandler) ServeCached(w http.ResponseWriter, r *http.Request, key, method, upstreamURL string) (bool, error) {
	if e, _ := ch.cache.Get(r.Context(), key); e != nil {
		// 缓存命中
		contentType := e.ContentType
		if contentType == "" {
			contentType = e.Headers["Content-Type"]
		}
		cacheControl := cache.GetCacheControlByContentType(contentType)
		w.Header().Set("Cache-Control", cacheControl)

		// 复制其他响应头（排除需要动态设置的头）
		for k, v := range e.Headers {
			lowerK := strings.ToLower(k)
			if k != "Cache-Control" && lowerK != "content-encoding" && lowerK != "content-length" {
				w.Header().Set(k, v)
			}
		}

		// 根据客户端需求动态压缩缓存的数据
		body := e.Body
		if method == http.MethodGet && len(body) > 0 {
			// 检查是否是视频/音频文件，这些文件通常已经压缩，不需要再次压缩
			isVideoOrAudio := strings.Contains(strings.ToLower(contentType), "video/") ||
				strings.Contains(strings.ToLower(contentType), "audio/")

			if !isVideoOrAudio {
				acceptEncoding := r.Header.Get("Accept-Encoding")
				compressedBody, encoding := compressBody(body, acceptEncoding)
				if encoding != "" {
					w.Header().Set("Content-Encoding", encoding)
					w.Header().Set("Vary", "Accept-Encoding")
					body = compressedBody
				}
			}
		}

		w.WriteHeader(e.StatusCode)
		if method == http.MethodGet && len(body) > 0 {
			_, _ = w.Write(body)
		}
		return true, nil
	}
	return false, nil
}

// CacheResponse 缓存响应
func (ch *CacheHandler) CacheResponse(ctx context.Context, key string, entry *cache.Entry) error {
	return ch.cache.Set(ctx, key, entry, ch.cfg.CacheTTL)
}

// ProcessUpstreamResponse 处理上游响应（解压、缓存、压缩）
func (ch *CacheHandler) ProcessUpstreamResponse(w http.ResponseWriter, r *http.Request, resp *http.Response, method, upstreamURL string) error {
	// 复制响应头
	headers := map[string]string{}
	contentType := ""
	contentLength := int64(0)
	contentEncoding := ""

	for k, vals := range resp.Header {
		if isHopByHopHeader(k) {
			continue
		}
		lowerK := strings.ToLower(k)

		if lowerK == "content-length" {
			if len(vals) > 0 {
				_, _ = fmt.Sscanf(vals[0], "%d", &contentLength)
			}
		}

		if lowerK == "content-encoding" && len(vals) > 0 {
			contentEncoding = vals[0]
		}

		if lowerK == "content-encoding" || lowerK == "content-length" {
			continue
		}
		if len(vals) > 0 {
			w.Header()[k] = vals
			headers[k] = vals[0]
			if k == "Content-Type" {
				contentType = vals[0]
			}
		}
	}

	// 设置优化的Cache-Control头
	cacheControl := cache.GetCacheControlByContentType(contentType)
	w.Header().Set("Cache-Control", cacheControl)

	// 读取响应体
	var body []byte
	if method == http.MethodGet {
		isVideoOrAudio := strings.Contains(strings.ToLower(contentType), "video/") ||
			strings.Contains(strings.ToLower(contentType), "audio/")

		// 判断是否需要流式传输
		if contentLength > ch.cfg.LargeFileThreshold && !isVideoOrAudio {
			// 大文件直接流式传输，不缓存
			w.Header().Set("Accept-Ranges", "bytes")
			for k, vals := range resp.Header {
				lowerK := strings.ToLower(k)
				if !isHopByHopHeader(k) && lowerK != "content-encoding" {
					w.Header()[k] = vals
				}
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
			return nil
		}

		// 小文件或视频/音频文件读入内存
		readLimit := ch.cfg.LargeFileThreshold
		if isVideoOrAudio {
			readLimit = ch.cfg.VideoFileThreshold
		}
		limitedReader := io.LimitReader(resp.Body, readLimit)
		var err error
		body, err = io.ReadAll(limitedReader)
		if err != nil {
			return err
		}

		// 如果上游返回了 gzip 压缩的数据，手动解压
		if contentEncoding == "gzip" {
			gzReader, err := gzip.NewReader(bytes.NewReader(body))
			if err == nil {
				defer gzReader.Close()
				decompressed, err := io.ReadAll(gzReader)
				if err == nil {
					body = decompressed
				}
			}
		}

		// 根据客户端需求压缩响应
		acceptEncoding := r.Header.Get("Accept-Encoding")
		compressedBody, encoding := compressBody(body, acceptEncoding)
		if encoding != "" {
			w.Header().Set("Content-Encoding", encoding)
			w.Header().Set("Vary", "Accept-Encoding")
			body = compressedBody
		}

		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	} else {
		w.WriteHeader(resp.StatusCode)
	}

	return nil
}

// compressBody 根据Accept-Encoding头压缩响应体
// 这是一个包级别的函数，可以被多个文件使用
func compressBody(body []byte, acceptEncoding string) ([]byte, string) {
	if len(body) < 1024 {
		return body, ""
	}

	if strings.Contains(acceptEncoding, "gzip") {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(body); err == nil {
			if err := gz.Close(); err == nil {
				if buf.Len() < len(body) {
					return buf.Bytes(), "gzip"
				}
			}
		}
	}

	return body, ""
}
