package proxy

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// 定义常量限制
const (
	MaxPathLength     = 2048  // URL 路径最大长度
	MaxHostLength     = 253   // 域名最大长度（RFC 1035）
	MaxQueryLength    = 4096  // 查询字符串最大长度
	MaxHeaderCount    = 100   // 最大 header 数量
	MaxHeaderValueSize = 8192 // 单个 header 值最大大小
	MaxHeaderNameSize = 128   // header 名称最大长度
)

// 定义危险字符模式（防止路径遍历、注入攻击等）
var (
	// 路径遍历模式
	pathTraversalPattern = regexp.MustCompile(`\.\.[/\\]`)
	// SQL 注入模式（基础检测）
	sqlInjectionPattern = regexp.MustCompile(`(?i)(['"]|;|\-\-|\/\*|\*\/|xp_|sp_|exec|execute|select|insert|update|delete|drop|union|script:)`)
	// Shell 命令注入模式（更精确的模式，避免误报正常查询参数）
	shellInjectionPattern = regexp.MustCompile(`;\s*(cat|ls|rm|wget|curl|nc|netcat|bash|sh|python|perl|ruby|node|php|cmd|powershell)|\|\s*(cat|ls|grep|find|wget|curl)|` + "`" + `[^` + "`" + `]*` + "`" + `|\$\(.*\)|\$\{.*\}`)
	// 危险协议
	dangerousSchemes = []string{"file:", "ftp:", "javascript:", "data:", "vbscript:", "mailto:"}
	// 主机名中的非法字符
	invalidHostChars = regexp.MustCompile(`[^a-zA-Z0-9.\-_\[\]:]`)
)

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// NewValidationError 创建验证错误
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// InputValidator 输入验证器
type InputValidator struct {
	// 可以在这里添加配置，例如允许的域名白名单等
}

// NewInputValidator 创建输入验证器
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidatePath 验证 URL 路径是否安全
func (v *InputValidator) ValidatePath(path string) error {
	// 检查路径长度
	if len(path) == 0 {
		return NewValidationError("path", "empty path")
	}
	if len(path) > MaxPathLength {
		return NewValidationError("path", "path too long")
	}

	// 去除前导斜杠
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return NewValidationError("path", "empty path after trim")
	}

	// 检查路径遍历攻击
	if pathTraversalPattern.MatchString(path) {
		return NewValidationError("path", "path traversal detected")
	}

	// 检查危险协议（直接在路径中的情况）
	pathLower := strings.ToLower(path)
	for _, scheme := range dangerousSchemes {
		if strings.Contains(pathLower, scheme) {
			return NewValidationError("path", "dangerous scheme detected: "+scheme)
		}
	}

	// 检查空字节攻击（防止字符串截断）
	if strings.Contains(path, "\x00") {
		return NewValidationError("path", "null byte detected")
	}

	// 检查控制字符
	for _, r := range path {
		if unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t' {
			return NewValidationError("path", "control character detected")
		}
		// 防止 Unicode homograph 攻击（类似字符）
		if r > 127 && !v.isSafeUnicode(r) {
			return NewValidationError("path", "unsafe unicode character")
		}
	}

	// 检查是否以 http:// 或 https:// 开头
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// 解析完整 URL 进行验证
		u, err := url.Parse(path)
		if err != nil {
			return NewValidationError("path", "invalid URL: "+err.Error())
		}
		return v.ValidateURL(u)
	}

	return nil
}

// ValidateHost 验证主机名是否安全
func (v *InputValidator) ValidateHost(host string) error {
	// 检查长度
	if len(host) == 0 {
		return NewValidationError("host", "empty host")
	}
	if len(host) > MaxHostLength {
		return NewValidationError("host", "host too long")
	}

	// 转换为小写用于检查
	hostLower := strings.ToLower(host)

	// 检查 localhost
	if hostLower == "localhost" || hostLower == "127.0.0.1" {
		return nil
	}

	// 检查是否包含非法字符
	if invalidHostChars.MatchString(host) {
		return NewValidationError("host", "invalid characters in host")
	}

	// 尝试解析为域名
	if strings.Contains(host, ":") {
		// 可能是 IPv6 地址或 host:port 格式
		if _, err := url.Parse("https://" + host); err != nil {
			return NewValidationError("host", "invalid host:port format")
		}
	} else {
		// 普通域名或 IPv4
		if _, err := url.Parse("https://" + host); err != nil {
			return NewValidationError("host", "invalid host format")
		}
	}

	// 检查是否以点开头或结尾
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return NewValidationError("host", "host cannot start or end with dot")
	}

	// 检查连续的点
	if strings.Contains(host, "..") {
		return NewValidationError("host", "host cannot contain consecutive dots")
	}

	// 检查是否是纯 IP 地址（可选的安全限制）
	// if v.isPureIP(host) {
	//     return NewValidationError("host", "direct IP access not allowed")
	// }

	return nil
}

// ValidateQuery 验证查询字符串是否安全
func (v *InputValidator) ValidateQuery(query string) error {
	if len(query) == 0 {
		return nil
	}

	// 检查长度
	if len(query) > MaxQueryLength {
		return NewValidationError("query", "query string too long")
	}

	// 检查 SQL 注入模式
	if sqlInjectionPattern.MatchString(query) {
		return NewValidationError("query", "potential SQL injection detected")
	}

	// 检查 Shell 命令注入
	if shellInjectionPattern.MatchString(query) {
		return NewValidationError("query", "potential command injection detected")
	}

	// 检查空字节
	if strings.Contains(query, "\x00") {
		return NewValidationError("query", "null byte detected")
	}

	// 尝试解析查询字符串
	values, err := url.ParseQuery(query)
	if err != nil {
		return NewValidationError("query", "invalid query string: "+err.Error())
	}

	// 验证每个参数
	for key, vals := range values {
		if len(key) > 256 {
			return NewValidationError("query", "parameter name too long")
		}
		for _, val := range vals {
			if len(val) > 2048 {
				return NewValidationError("query", "parameter value too long")
			}
		}
	}

	return nil
}

// ValidateURL 验证完整的 URL 是否安全
func (v *InputValidator) ValidateURL(u *url.URL) error {
	if u == nil {
		return NewValidationError("url", "nil URL")
	}

	// 检查协议
	if u.Scheme != "http" && u.Scheme != "https" {
		return NewValidationError("url", "unsupported scheme: "+u.Scheme)
	}

	// 验证主机
	if err := v.ValidateHost(u.Host); err != nil {
		return err
	}

	// 验证路径
	if u.Path != "" {
		if err := v.ValidatePath(u.Path); err != nil {
			// 对于完整 URL，放宽一些路径检查
			if !strings.Contains(err.Error(), "path traversal") &&
				!strings.Contains(err.Error(), "null byte") {
				return err
			}
		}
	}

	// 验证查询参数
	if u.RawQuery != "" {
		if err := v.ValidateQuery(u.RawQuery); err != nil {
			return err
		}
	}

	return nil
}

// ValidateHeaders 验证 HTTP 头是否安全
func (v *InputValidator) ValidateHeaders(headers map[string][]string) error {
	// 检查 header 数量
	if len(headers) > MaxHeaderCount {
		return NewValidationError("headers", "too many headers")
	}

	for name, values := range headers {
		// 检查 header 名称不能为空
		if name == "" {
			return NewValidationError("header", "empty header name")
		}

		// 检查 header 名称
		if len(name) > MaxHeaderNameSize {
			return NewValidationError("header", "header name too long: "+name)
		}

		// 检查 header 名称中的非法字符
		for _, r := range name {
			if r < 32 || r > 126 || r == ':' || r == '\r' || r == '\n' {
				return NewValidationError("header", "invalid character in header name: "+name)
			}
		}

		// 检查每个值
		for _, value := range values {
			if len(value) > MaxHeaderValueSize {
				return NewValidationError("header", "header value too long: "+name)
			}

			// 检查控制字符（除了 HTAB）
			for _, r := range value {
				if unicode.IsControl(r) && r != '\t' {
					return NewValidationError("header", "control character in header value: "+name)
				}
			}
		}
	}

	return nil
}

// SanitizeString 清理字符串中的危险内容
func (v *InputValidator) SanitizeString(s string) string {
	// 移除空字节
	s = strings.ReplaceAll(s, "\x00", "")

	// 移除控制字符（保留换行和制表符）
	var result strings.Builder
	for _, r := range s {
		if !unicode.IsControl(r) || r == '\r' || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// isSafeUnicode 检查 Unicode 字符是否安全（防止 homograph 攻击）
func (v *InputValidator) isSafeUnicode(r rune) bool {
	// 基本拉丁字母、数字、常用符号
	if r <= 0x024F {
		return true
	}

	// 中文、日文、韩文等 CJK 字符
	if (r >= 0x4E00 && r <= 0x9FFF) || // CJK 统一表意文字
		(r >= 0x3040 && r <= 0x309F) || // 平假名
		(r >= 0x30A0 && r <= 0x30FF) || // 片假名
		(r >= 0xAC00 && r <= 0xD7AF) { // 韩文
		return true
	}

	// 常用标点和符号
	if (r >= 0x2000 && r <= 0x206F) || // 通用标点
		(r >= 0x3000 && r <= 0x303F) { // CJK 标点
		return true
	}

	// 其他字符视为不安全（需要根据实际需求调整）
	return false
}

// isPureIP 检查是否是纯 IP 地址
func (v *InputValidator) isPureIP(host string) bool {
	// IPv4 模式
	ipv4Pattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}(:\d+)?$`)
	if ipv4Pattern.MatchString(host) {
		return true
	}

	// IPv6 模式（简化检查）
	if strings.Contains(host, ":") && !strings.Contains(host, ".") {
		return true
	}

	return false
}

// ValidateUpstreamURL 验证上游 URL（代理场景专用）
func (v *InputValidator) ValidateUpstreamURL(upstreamURL string) error {
	if upstreamURL == "" {
		return errors.New("empty upstream URL")
	}

	// 检查长度
	if len(upstreamURL) > MaxPathLength+MaxQueryLength {
		return NewValidationError("upstream_url", "URL too long")
	}

	// 解析 URL
	u, err := url.Parse(upstreamURL)
	if err != nil {
		return NewValidationError("upstream_url", "invalid URL: "+err.Error())
	}

	// 完整验证
	return v.ValidateURL(u)
}
