package httpapi

import (
	"errors"
	"net/http"

	"github.com/mcpic/mcpic/internal/meta"
)

// galleryImage 是 /api/gallery JSON 列表项（D4.7 P4：列表数据只经鉴权 API 暴露）。
type galleryImage struct {
	Key      string `json:"key"`
	Format   string `json:"format"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Size     int64  `json:"size"`
	Created  string `json:"created"`
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	BBCode   string `json:"bbcode"`
}

// handleUploadPage 渲染上传页（拖拽 + 粘贴 + 多选队列，PRD 9）。
func (s *Server) handleUploadPage(w http.ResponseWriter, _ *http.Request) {
	s.writeStatic(w, "static/upload.html")
}

// handleGallery 返回画廊壳页（D4.7 P4）：页面本身不含任何图片数据，
// 列表由前端持 localStorage 中的 API Key 调用 /api/gallery 渲染；
// 无 Key 时前端强制跳转 /upload。
func (s *Server) handleGallery(w http.ResponseWriter, _ *http.Request) {
	s.writeStatic(w, "static/gallery.html")
}

// handleAPIGallery 实现 GET /api/gallery（withAuth 保护）：返回最近图片列表。
func (s *Server) handleAPIGallery(w http.ResponseWriter, r *http.Request) {
	imgs, err := s.svc.ListImages(r.Context(), 100)
	if err != nil {
		writeAppError(w, err)
		return
	}
	base := getBaseURL(s.cfg, r)
	items := make([]galleryImage, 0, len(imgs))
	for _, img := range imgs {
		items = append(items, s.galleryImage(img, base))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// galleryImage 组装单张图片的列表数据。
func (s *Server) galleryImage(img meta.Image, base string) galleryImage {
	rel := s.svc.URL(img.Key)
	url := base + rel
	return galleryImage{
		Key:      img.Key,
		Format:   img.Format,
		Width:    img.Width,
		Height:   img.Height,
		Size:     img.Size,
		Created:  img.CreatedAt.Local().Format("2006-01-02 15:04"),
		URL:      url,
		Markdown: "![image](" + url + ")",
		HTML:     `<img src="` + url + `" alt="image">`,
		BBCode:   "[img]" + url + "[/img]",
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
