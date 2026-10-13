package httpapi

import (
	"errors"
	"net/http"

	"github.com/mcpic/mcpic/internal/meta"
)

// galleryItem 是画廊页一张卡片的模板数据（含四格式复制内容）。
type galleryItem struct {
	Key        string
	Format     string
	Width      int
	Height     int
	Size       string
	Created    string
	RawURL     string
	URL        string
	Markdown   string
	HTML       string
	BBCode     string
	JSURL      string
	JSMarkdown string
	JSHTML     string
	JSBBCode   string
}

// galleryData 是画廊页模板数据。
type galleryData struct {
	Items []galleryItem
	Count int
}

// handleUploadPage 渲染上传页（拖拽 + 粘贴，PRD 9）。
func (s *Server) handleUploadPage(w http.ResponseWriter, _ *http.Request) {
	s.writeStatic(w, "static/upload.html")
}

// handleGallery 渲染画廊页：网格展示最近上传的图片（PRD 9）。
func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	imgs, err := s.svc.ListImages(r.Context(), 100)
	if err != nil {
		writeAppError(w, err)
		return
	}
	base := getBaseURL(s.cfg, r)
	data := galleryData{Items: make([]galleryItem, 0, len(imgs)), Count: len(imgs)}
	for _, img := range imgs {
		data.Items = append(data.Items, s.galleryItem(img, base))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if err := galleryPageTmpl.Execute(w, data); err != nil {
		logTemplateError("gallery", err)
	}
}

// galleryItem 组装单张卡片数据。
func (s *Server) galleryItem(img meta.Image, base string) galleryItem {
	rel := s.svc.URL(img.Key)
	url := base + rel
	md := "![image](" + url + ")"
	html := `<img src="` + url + `" alt="image">`
	bb := "[img]" + url + "[/img]"
	return galleryItem{
		Key: img.Key, Format: img.Format,
		Width: img.Width, Height: img.Height,
		Size:    humanSize(img.Size),
		Created: img.CreatedAt.Local().Format("2006-01-02 15:04"),
		RawURL:  rel + "?raw=1",
		URL:     url, Markdown: md, HTML: html, BBCode: bb,
		JSURL: url, JSMarkdown: md, JSHTML: html, JSBBCode: bb,
	}
}

// handleDeleteImage 实现 DELETE /api/images/{key}（withAuth 保护，PRD 9 画廊删除）。
func (s *Server) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := s.svc.DeleteImage(r.Context(), key); err != nil {
		if errors.Is(err, meta.ErrNotFound) {
			writeAppError(w, err)
			return
		}
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// writeStatic 直出 embed 中的静态页面。
func (s *Server) writeStatic(w http.ResponseWriter, name string) {
	data, err := staticFS.ReadFile(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "STORAGE_ERROR",
			Message: "内嵌页面资源缺失：请重新构建二进制",
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
