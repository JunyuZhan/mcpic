// Package config 读取 MCPic 的环境变量配置。
// PRD 第 10 节：v1 只需要四个环境变量，没有 YAML，没有配置文件。
package config

import (
	"fmt"
	"os"
	"strconv"
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
	// StorageLocal / StorageS3 是 MCPIC_STORAGE_TYPE 的合法取值。
	StorageLocal = "local"
	StorageS3    = "s3"
)

// S3Config 是 S3 兼容对象存储的配置（v2）。
type S3Config struct {
	Endpoint        string // 兼容端点，如 https://<account_id>.r2.cloudflarestorage.com
	Region          string // 区域，R2 用 auto
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string // 直链前缀（CDN / r2.dev 域名），不含尾部斜杠
	ForcePathStyle  bool   // path-style 访问，默认 true（兼容 R2/MinIO/COS/OSS）
}

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
	// StorageType 为 MCPIC_STORAGE_TYPE：local（默认）或 s3。
	StorageType string
	// S3 为对象存储配置，仅 StorageType=s3 时使用。
	S3 S3Config
}

// IsS3 表示是否使用对象存储后端。
func (c *Config) IsS3() bool { return c.StorageType == StorageS3 }

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

	cfg.StorageType = os.Getenv("MCPIC_STORAGE_TYPE")
	cfg.S3 = S3Config{
		Endpoint:        strings.TrimRight(os.Getenv("MCPIC_S3_ENDPOINT"), "/"),
		Region:          os.Getenv("MCPIC_S3_REGION"),
		Bucket:          os.Getenv("MCPIC_S3_BUCKET"),
		AccessKeyID:     os.Getenv("MCPIC_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("MCPIC_S3_SECRET_ACCESS_KEY"),
		PublicBaseURL:   strings.TrimRight(os.Getenv("MCPIC_S3_PUBLIC_BASE_URL"), "/"),
		ForcePathStyle:  envBoolDefault("MCPIC_S3_FORCE_PATH_STYLE", true),
	}
	return cfg
}

// Validate 校验配置。本地模式恒通过；s3 模式要求必填项齐全（fail fast，宪法 7.5）。
func (c *Config) Validate() error {
	switch c.StorageType {
	case "", StorageLocal:
		return nil
	case StorageS3:
		s := c.S3
		missing := make([]string, 0, 6)
		if s.Endpoint == "" {
			missing = append(missing, "MCPIC_S3_ENDPOINT")
		}
		if s.Region == "" {
			missing = append(missing, "MCPIC_S3_REGION")
		}
		if s.Bucket == "" {
			missing = append(missing, "MCPIC_S3_BUCKET")
		}
		if s.AccessKeyID == "" {
			missing = append(missing, "MCPIC_S3_ACCESS_KEY_ID")
		}
		if s.SecretAccessKey == "" {
			missing = append(missing, "MCPIC_S3_SECRET_ACCESS_KEY")
		}
		if s.PublicBaseURL == "" {
			missing = append(missing, "MCPIC_S3_PUBLIC_BASE_URL")
		}
		if len(missing) > 0 {
			return fmt.Errorf("S3 模式缺少必填环境变量：%s", strings.Join(missing, ", "))
		}
		return nil
	default:
		return fmt.Errorf("MCPIC_STORAGE_TYPE 非法值 %q，支持：local、s3", c.StorageType)
	}
}

// envBoolDefault 读取布尔环境变量，空值时返回默认值。
func envBoolDefault(name string, def bool) bool {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
