package httpapi

import (
	"net/http"
	"strings"

	"github.com/mcpic/mcpic/internal/config"
)

// joinURL 拼接链接：u 已是绝对 URL（http/https 开头，S3 直连）则直接用；
// 否则视为相对路径，接在 base 之后（本地代理模式）。
func joinURL(base, u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return base + u
}

// getBaseURL 按 PRD 3.4 的四级回退生成链接基础地址，顺序不可颠倒（实现提示 4）：
//  1. MCPIC_BASE_URL（显式配置优先，无条件使用）
//  2. X-Forwarded-Host（反向代理场景）
//  3. Host（直接访问场景）
//  4. http://localhost:8080（兜底）
func getBaseURL(cfg *config.Config, r *http.Request) string {
	if cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		scheme := r.Header.Get("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "https"
		}
		return scheme + "://" + fwdHost
	}
	if r.Host != "" {
		return "http://" + r.Host
	}
	return "http://localhost:8080"
}
