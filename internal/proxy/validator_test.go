package proxy

import (
	"fmt"
	"net/url"
	"testing"
)

// 测试 ValidatePath
func TestValidatePath_ValidPaths(t *testing.T) {
	validator := NewInputValidator()

	validPaths := []string{
		"/cdn.jsdelivr.net/npm/bootstrap",
		"/unpkg.com/react@18/umd/react.production.min.js",
		"/https://example.com/path/to/resource",
		"/http://api.example.com/v1/data",
	}

	for _, path := range validPaths {
		t.Run(path, func(t *testing.T) {
			err := validator.ValidatePath(path)
			if err != nil {
				t.Errorf("ValidatePath(%q) = %v, want nil", path, err)
			}
		})
	}
}

func TestValidatePath_PathTraversal(t *testing.T) {
	validator := NewInputValidator()

	pathTraversalCases := []string{
		"/../../../etc/passwd",
		"/cdn.jsdelivr.net/../../../etc/passwd",
		"/..\\windows\\system32",
		"/unpkg.com/../../sensitive.txt",
	}

	for _, path := range pathTraversalCases {
		t.Run(path, func(t *testing.T) {
			err := validator.ValidatePath(path)
			if err == nil {
				t.Errorf("ValidatePath(%q) = nil, want error (path traversal)", path)
			}
		})
	}
}

func TestValidatePath_NullByte(t *testing.T) {
	validator := NewInputValidator()

	nullByteCases := []string{
		"/cdn.jsdelivr.net\x00.js",
		"/unpkg.com\x00/react",
		"/test\x00/path",
	}

	for _, path := range nullByteCases {
		t.Run(path, func(t *testing.T) {
			err := validator.ValidatePath(path)
			if err == nil {
				t.Errorf("ValidatePath(%q) = nil, want error (null byte)", path)
			}
		})
	}
}

func TestValidatePath_ControlCharacters(t *testing.T) {
	validator := NewInputValidator()

	controlCases := []string{
		"/cdn.jsdelivr.net\x01.js",
		"/unpkg.com\x02/react",
	}

	for _, path := range controlCases {
		t.Run(path, func(t *testing.T) {
			err := validator.ValidatePath(path)
			if err == nil {
				t.Errorf("ValidatePath(%q) = nil, want error (control char)", path)
			}
		})
	}
}

func TestValidatePath_TooLong(t *testing.T) {
	validator := NewInputValidator()

	longPath := "/" + string(make([]byte, MaxPathLength+1))
	err := validator.ValidatePath(longPath)
	if err == nil {
		t.Error("ValidatePath(long path) = nil, want error (too long)")
	}
}

func TestValidatePath_Empty(t *testing.T) {
	validator := NewInputValidator()

	err := validator.ValidatePath("")
	if err == nil {
		t.Error("ValidatePath(\"\") = nil, want error (empty)")
	}
}

// 测试 ValidateHost
func TestValidateHost_ValidHosts(t *testing.T) {
	validator := NewInputValidator()

	validHosts := []string{
		"cdn.jsdelivr.net",
		"unpkg.com",
		"example.com",
		"sub.domain.example.com",
		"localhost",
		"api.example.com:8080",
		"127.0.0.1",
		"example.co.uk",
	}

	for _, host := range validHosts {
		t.Run(host, func(t *testing.T) {
			err := validator.ValidateHost(host)
			if err != nil {
				t.Errorf("ValidateHost(%q) = %v, want nil", host, err)
			}
		})
	}
}

func TestValidateHost_InvalidHosts(t *testing.T) {
	validator := NewInputValidator()

	invalidHosts := []string{
		"",
		".example.com",
		"example..com",
		"example.com.",
		"../../../etc",
		"exa mple.com",
		"example.com<script>",
	}

	for _, host := range invalidHosts {
		t.Run(host, func(t *testing.T) {
			err := validator.ValidateHost(host)
			if err == nil {
				t.Errorf("ValidateHost(%q) = nil, want error", host)
			}
		})
	}
}

func TestValidateHost_TooLong(t *testing.T) {
	validator := NewInputValidator()

	longHost := string(make([]byte, MaxHostLength+1))
	err := validator.ValidateHost(longHost)
	if err == nil {
		t.Error("ValidateHost(long host) = nil, want error (too long)")
	}
}

// 测试 ValidateQuery
func TestValidateQuery_ValidQueries(t *testing.T) {
	validator := NewInputValidator()

	validQueries := []string{
		"v=1.0&key=value",
		"search=test&page=1",
		"filter=name&sort=asc",
		"url=https://example.com",
		"",
	}

	for _, query := range validQueries {
		t.Run(query, func(t *testing.T) {
			err := validator.ValidateQuery(query)
			if err != nil {
				t.Errorf("ValidateQuery(%q) = %v, want nil", query, err)
			}
		})
	}
}

func TestValidateQuery_SQLInjection(t *testing.T) {
	validator := NewInputValidator()

	sqlInjectionCases := []string{
		"id=1' OR '1'='1",
		"query=test'; DROP TABLE users; --",
		"name=admin' UNION SELECT * FROM users--",
		"data=1'; EXEC xp_cmdshell('dir'); --",
	}

	for _, query := range sqlInjectionCases {
		t.Run(query, func(t *testing.T) {
			err := validator.ValidateQuery(query)
			if err == nil {
				t.Errorf("ValidateQuery(%q) = nil, want error (SQL injection)", query)
			}
		})
	}
}

func TestValidateQuery_CommandInjection(t *testing.T) {
	validator := NewInputValidator()

	cmdInjectionCases := []string{
		"file=test.txt; cat /etc/passwd",
		"path=/tmp; rm -rf /",
		"cmd=ls|grep secret",
		"data=`whoami`",
	}

	for _, query := range cmdInjectionCases {
		t.Run(query, func(t *testing.T) {
			err := validator.ValidateQuery(query)
			if err == nil {
				t.Errorf("ValidateQuery(%q) = nil, want error (command injection)", query)
			}
		})
	}
}

func TestValidateQuery_TooLong(t *testing.T) {
	validator := NewInputValidator()

	longQuery := "key=" + string(make([]byte, MaxQueryLength+1))
	err := validator.ValidateQuery(longQuery)
	if err == nil {
		t.Error("ValidateQuery(long query) = nil, want error (too long)")
	}
}

// 测试 ValidateURL
func TestValidateURL_ValidURLs(t *testing.T) {
	validator := NewInputValidator()

	validURLs := []string{
		"https://cdn.jsdelivr.net/npm/bootstrap",
		"http://api.example.com/v1/data",
		"https://example.com:8080/path?query=value",
		"https://sub.domain.example.com/resource",
	}

	for _, urlString := range validURLs {
		t.Run(urlString, func(t *testing.T) {
			u, err := url.Parse(urlString)
			if err != nil {
				t.Fatalf("url.Parse(%q) = %v", urlString, err)
			}
			err = validator.ValidateURL(u)
			if err != nil {
				t.Errorf("ValidateURL(%q) = %v, want nil", urlString, err)
			}
		})
	}
}

func TestValidateURL_InvalidSchemes(t *testing.T) {
	validator := NewInputValidator()

	invalidSchemes := []string{
		"file:///etc/passwd",
		"ftp://example.com/file",
		"javascript:alert('xss')",
		"data:text/html,<script>alert('xss')</script>",
	}

	for _, urlString := range invalidSchemes {
		t.Run(urlString, func(t *testing.T) {
			u, err := url.Parse(urlString)
			if err != nil {
				t.Fatalf("url.Parse(%q) = %v", urlString, err)
			}
			err = validator.ValidateURL(u)
			if err == nil {
				t.Errorf("ValidateURL(%q) = nil, want error (invalid scheme)", urlString)
			}
		})
	}
}

func TestValidateURL_InvalidHost(t *testing.T) {
	validator := NewInputValidator()

	invalidCases := []string{
		"https://../../../etc/passwd",
		"https://example.com<script>/path",
	}

	for _, urlString := range invalidCases {
		t.Run(urlString, func(t *testing.T) {
			u, err := url.Parse(urlString)
			if err != nil {
				t.Fatalf("url.Parse(%q) = %v", urlString, err)
			}
			err = validator.ValidateURL(u)
			if err == nil {
				t.Errorf("ValidateURL(%q) = nil, want error", urlString)
			}
		})
	}
}

// 测试 ValidateHeaders
func TestValidateHeaders_ValidHeaders(t *testing.T) {
	validator := NewInputValidator()

	validHeaders := map[string][]string{
		"Content-Type":        {"application/json"},
		"Authorization":       {"Bearer token123"},
		"User-Agent":          {"Mozilla/5.0"},
		"Accept":              {"application/json", "text/html"},
		"X-Custom-Header":     {"value"},
	}

	err := validator.ValidateHeaders(validHeaders)
	if err != nil {
		t.Errorf("ValidateHeaders() = %v, want nil", err)
	}
}

func TestValidateHeaders_TooManyHeaders(t *testing.T) {
	validator := NewInputValidator()

	headers := make(map[string][]string)
	for i := 0; i < MaxHeaderCount+1; i++ {
		headers[fmt.Sprintf("Header-%d", i)] = []string{"value"}
	}

	err := validator.ValidateHeaders(headers)
	if err == nil {
		t.Error("ValidateHeaders(too many) = nil, want error")
	}
}

func TestValidateHeaders_InvalidHeaderName(t *testing.T) {
	validator := NewInputValidator()

	invalidCases := []map[string][]string{
		{"Header\nName": {"value"}},
		{"Header\rName": {"value"}},
		{"Header:Name": {"value"}},
		{"": {"value"}},
	}

	for i, headers := range invalidCases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			err := validator.ValidateHeaders(headers)
			if err == nil {
				t.Error("ValidateHeaders(invalid name) = nil, want error")
			}
		})
	}
}

func TestValidateHeaders_TooLongValue(t *testing.T) {
	validator := NewInputValidator()

	headers := map[string][]string{
		"Long-Header": {string(make([]byte, MaxHeaderValueSize+1))},
	}

	err := validator.ValidateHeaders(headers)
	if err == nil {
		t.Error("ValidateHeaders(too long value) = nil, want error")
	}
}

// 测试 ValidateUpstreamURL
func TestValidateUpstreamURL_Valid(t *testing.T) {
	validator := NewInputValidator()

	validURLs := []string{
		"https://cdn.jsdelivr.net/npm/bootstrap",
		"http://api.example.com/v1/data",
		"https://example.com:8080/path",
	}

	for _, urlStr := range validURLs {
		t.Run(urlStr, func(t *testing.T) {
			err := validator.ValidateUpstreamURL(urlStr)
			if err != nil {
				t.Errorf("ValidateUpstreamURL(%q) = %v, want nil", urlStr, err)
			}
		})
	}
}

func TestValidateUpstreamURL_Invalid(t *testing.T) {
	validator := NewInputValidator()

	invalidURLs := []string{
		"",
		"not a url",
		"file:///etc/passwd",
		"https://../../../etc/passwd",
	}

	for _, urlStr := range invalidURLs {
		t.Run(urlStr, func(t *testing.T) {
			err := validator.ValidateUpstreamURL(urlStr)
			if err == nil {
				t.Errorf("ValidateUpstreamURL(%q) = nil, want error", urlStr)
			}
		})
	}
}

// 测试 SanitizeString
func TestSanitizeString(t *testing.T) {
	validator := NewInputValidator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "null byte",
			input:    "test\x00string",
			expected: "teststring",
		},
		{
			name:     "control characters",
			input:    "test\x01\x02string",
			expected: "teststring",
		},
		{
			name:     "preserve newlines and tabs",
			input:    "test\n\tstring",
			expected: "test\n\tstring",
		},
		{
			name:     "normal string",
			input:    "normal string",
			expected: "normal string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试 ValidationError
func TestValidationError(t *testing.T) {
	err := NewValidationError("field", "error message")
	expected := "field: error message"

	if err.Error() != expected {
		t.Errorf("ValidationError.Error() = %q, want %q", err.Error(), expected)
	}

	if err.Field != "field" {
		t.Errorf("ValidationError.Field = %q, want %q", err.Field, "field")
	}

	if err.Message != "error message" {
		t.Errorf("ValidationError.Message = %q, want %q", err.Message, "error message")
	}
}
