package httpapi

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/mcpic/mcpic/internal/meta"
)

//go:embed static/*.html
var staticFS embed.FS

var imagePageTmpl = template.Must(template.ParseFS(staticFS, "static/image.html"))
var homePageTmpl = template.Must(template.ParseFS(staticFS, "static/index.html"))

// handleHome 首页：项目说明、快速开始、MCP 配置示例、API 速查（PRD 第 9 节）。
// D4.7 P3：MCP 配置示例的 BASE_URL 由服务端注入（配置值或请求推断），禁止硬编码域名。
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if err := homePageTmpl.Execute(w, map[string]string{"BaseURL": getBaseURL(s.cfg, r)}); err != nil {
		logTemplateError("home", err)
	}
}

// imagePageData 是图片页模板数据。
type imagePageData struct {
	Key        string
	Format     string
	Width      int
	Height     int
	Size       string
	URL        string
	RawURL     string
	Markdown   string
	JSMarkdown string
	JSURL      string
}

// wantsHTML 判断请求是否应返回预览页：浏览器导航带 Accept: text/html，
// 渲染器 / curl / API 客户端不带 → 仍拿到原始图片字节。
func wantsHTML(r *http.Request) bool {
	if r.URL.Query().Get("raw") == "1" {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// renderImagePage 输出图片预览页（展示图片 + 一键复制 Markdown）。
func (s *Server) renderImagePage(w http.ResponseWriter, r *http.Request, img meta.Image) {
	rel := s.svc.URL(img.Key)
	url := joinURL(getBaseURL(s.cfg, r), rel)
	// 直连模式下 rel 已是绝对直链，?raw=1 无意义；本地模式需拼 ?raw=1 取原图。
	rawURL := rel
	if !strings.HasPrefix(rel, "http://") && !strings.HasPrefix(rel, "https://") {
		rawURL = rel + "?raw=1"
	}
	data := imagePageData{
		Key:        img.Key,
		Format:     img.Format,
		Width:      img.Width,
		Height:     img.Height,
		Size:       humanSize(img.Size),
		URL:        url,
		RawURL:     rawURL,
		Markdown:   "![image](" + url + ")",
		JSMarkdown: "![image](" + url + ")",
		JSURL:      url,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if err := imagePageTmpl.Execute(w, data); err != nil {
		// 响应已开始，只能记录（宪法 7.4/7.5）。
		logTemplateError(img.Key, err)
	}
}

// humanSize 格式化字节大小（如 "180KB"）。
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + "B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB"}
	s := strconv.FormatFloat(float64(n)/float64(div), 'f', 1, 64)
	if strings.HasSuffix(s, ".0") {
		s = strings.TrimSuffix(s, ".0")
	}
	return s + units[exp]
}
