package httpapi

import (
	"crypto/subtle"
	"net/http"

	"github.com/mcpic/mcpic/internal/apperr"
	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/service"
)

// Server 组装 HTTP 路由（PRD 3.2）。
type Server struct {
	cfg *config.Config
	svc *service.Service
}

// New 构造 Server。
func New(cfg *config.Config, svc *service.Service) *Server {
	return &Server{cfg: cfg, svc: svc}
}

// Routes 返回全部路由。
// 未实现的 /metrics 属于 D4 范围，此处不注册（宪法 4.4：不留假桩）。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /api/upload", s.withAuth(s.handleUpload))
	mux.HandleFunc("GET /i/{key}", s.handleImage)
	return mux
}

// withAuth 校验 X-API-Key。未配置 MCPIC_API_KEY 时放行（本地零配置启动）。
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.APIKey != "" {
			got := r.Header.Get("X-API-Key")
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.APIKey)) != 1 {
				writeAppError(w, apperr.Unauthorized())
				return
			}
		}
		next(w, r)
	}
}

// handleHealthz 健康检查。
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
