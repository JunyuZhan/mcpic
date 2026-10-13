package httpapi

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/mcpic/mcpic/internal/meta"
)

//go:embed static/index.html static/image.html
var staticFS embed.FS

var imagePageTmpl = template.Must(template.ParseFS(staticFS, "static/image.html"))

// handleHome 首页：项目说明、快速开始、MCP 配置示例、API 速查（PRD 第 9 节）。
func (s *Server) handleHome(w http.ResponseWriter, _ *http.Request) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		// embed 的文件不可能读失败；真失败时给出可执行指引而非模糊错误。
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "STORAGE_ERROR",
			Message: "内嵌首页资源缺失：请重新构建二进制",
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
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
	url := getBaseURL(s.cfg, r) + s.svc.URL(img.Key)
	data := imagePageData{
		Key:        img.Key,
		Format:     img.Format,
		Width:      img.Width,
		Height:     img.Height,
		Size:       humanSize(img.Size),
		URL:        url,
		RawURL:     s.svc.URL(img.Key) + "?raw=1",
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
