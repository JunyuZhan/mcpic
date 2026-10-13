// Package config 读取 MCPic 的环境变量配置。
// PRD 第 10 节：v1 只需要四个环境变量，没有 YAML，没有配置文件。
package config

import (
	"os"
	"strings"
)

const (
	// DefaultStoragePath 图片存储路径默认值（本地开发）。
	DefaultStoragePath = "./data"
	// DefaultAllowedPaths 本地路径白名单默认值（PRD 3.5）。
	DefaultAllowedPaths = "/tmp:/home:/Users"
	// DefaultAddr 监听地址。PRD 未提供配置项，固定为 :8080。
	DefaultAddr = ":8080"
	// MaxUploadBytes 上传大小限制 10MB（PRD 第 4 节 FILE_TOO_LARGE 示例）。
	MaxUploadBytes int64 = 10 << 20
)

// Config 是服务的全部运行时配置。
type Config struct {
	// APIKey 为 MCPIC_API_KEY。空字符串表示不启用鉴权（本地零配置启动）。
	APIKey string
	// StoragePath 为 MCPIC_STORAGE_PATH，SQLite 数据库自动创建在该目录下。
	StoragePath string
	// BaseURL 为 MCPIC_BASE_URL。空字符串表示按四级回退从请求推断。
	BaseURL string
	// AllowedPaths 为 MCPIC_ALLOWED_PATHS，按 ":" 分隔；AllowAll 为 true 时放行所有路径。
	AllowedPaths []string
	AllowAll     bool
	// Addr 为监听地址。
	Addr string
	// MaxBytes 为单次上传大小上限。
	MaxBytes int64
}

// Load 从环境变量构造 Config。
func Load() *Config {
	cfg := &Config{
		APIKey:      os.Getenv("MCPIC_API_KEY"),
		StoragePath: os.Getenv("MCPIC_STORAGE_PATH"),
		BaseURL:     strings.TrimRight(os.Getenv("MCPIC_BASE_URL"), "/"),
		Addr:        DefaultAddr,
		MaxBytes:    MaxUploadBytes,
	}

	if cfg.StoragePath == "" {
		cfg.StoragePath = DefaultStoragePath
	}

	allowed := os.Getenv("MCPIC_ALLOWED_PATHS")
	if allowed == "" {
		allowed = DefaultAllowedPaths
	}
	if allowed == "*" {
		cfg.AllowAll = true
	} else {
		for _, p := range strings.Split(allowed, ":") {
			if p = strings.TrimSpace(p); p != "" {
				cfg.AllowedPaths = append(cfg.AllowedPaths, p)
			}
		}
	}
	return cfg
}
