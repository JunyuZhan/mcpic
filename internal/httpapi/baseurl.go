package httpapi

import (
	"net/http"

	"github.com/mcpic/mcpic/internal/config"
)

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
