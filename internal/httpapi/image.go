package httpapi

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/mcpic/mcpic/internal/imgproc"
)

// handleImage 实现 GET /i/:key，返回图片内容（PRD 3.2）。
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	img, rc, err := s.svc.Get(r.Context(), key)
	if err != nil {
		writeAppError(w, err)
		return
	}
	defer func() { _ = rc.Close() }()

	// 宪法 5.5：正确的 Content-Type 与 Content-Disposition，
	// 且不把用户上传内容当可执行文件暴露。
	w.Header().Set("Content-Type", imgproc.ContentType(img.Format))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", key+"."+img.Format))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		// 响应头已发出，只能记录，不能再改状态码（宪法 7.4/7.5）。
		log.Printf("写图片响应失败 key=%s: %v", key, err)
	}
}
