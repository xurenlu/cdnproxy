package docs

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed llm.txt
var llmTxtContent []byte

//go:embed llms.txt
var llmsTxtContent []byte

// LLMTxtHandler 返回 /llm.txt 内容（简洁版，供 AI 快速理解）
func LLMTxtHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		base := "https://" + r.Host
		content := strings.ReplaceAll(string(llmTxtContent), "{{BASE}}", base)
		_, _ = w.Write([]byte(content))
	}
}

// LLMsTxtHandler 返回 /llms.txt 内容（完整版，符合 llms.txt 规范）
func LLMsTxtHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// 根据请求 Host 动态替换 base URL
		base := "https://" + r.Host
		content := strings.ReplaceAll(string(llmsTxtContent), "{{BASE}}", base)
		_, _ = w.Write([]byte(content))
	}
}
