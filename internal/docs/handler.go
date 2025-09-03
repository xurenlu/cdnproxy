package docs

import (
	_ "embed"
	"net/http"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

//go:embed public_usage.md
var docMarkdown []byte

func Handler() http.HandlerFunc {
	htmlFlags := html.CommonFlags | html.CompletePage | html.HrefTargetBlank
	opts := html.RendererOptions{
		Flags: htmlFlags,
		Title: "CDNProxy Usage",
	}
	renderer := html.NewRenderer(opts)

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)

	docHTML := markdown.ToHTML(docMarkdown, p, renderer)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(docHTML)
	}
}
