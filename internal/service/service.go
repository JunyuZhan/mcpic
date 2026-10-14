// Package service 组合存储、元数据与图片识别，供 HTTP 与 MCP 两层复用。
// HTTP Handler 不直接访问数据库（宪法 8.1）。
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mcpic/mcpic/internal/apperr"
	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/imgproc"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/storage"
)

// Service 是上传与读取用例。
type Service struct {
	cfg *config.Config
	st  storage.Storage
	db  *meta.Store
}

// New 构造 Service。
func New(cfg *config.Config, st storage.Storage, db *meta.Store) *Service {
	return &Service{cfg: cfg, st: st, db: db}
}

// Result 是一次成功上传的元信息。
type Result struct {
	Key    string
	Format string
	Size   int64
	Width  int
	Height int
}

// URL 返回图片的相对路径（完整链接由 HTTP 层按 PRD 3.4 拼接）。
func (s *Service) URL(key string) string { return s.st.URL(key) }

// Options 是上传处理参数（PRD 3.1）。
type Options struct {
	// Format 为 webp（默认）或 original。
	Format string
	// MaxWidth 为最大宽度，默认 1920。
	MaxWidth int
}

// Upload 从任意 reader 读取图片，按 PRD 第 5 节处理后落盘。
// filename 仅用于生成更有意义的格式错误提示，不参与落盘命名。
func (s *Service) Upload(ctx context.Context, r io.Reader, filename string, opts Options) (Result, error) {
	limited := io.LimitReader(r, s.cfg.MaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, apperr.StorageError(err)
	}
	if int64(len(data)) > s.cfg.MaxBytes {
		return Result{}, apperr.FileTooLarge(formatSize(int64(len(data))))
	}

	// 魔数识别（实现提示 2），不看后缀名。
	if _, err := imgproc.Detect(data); err != nil {
		return Result{}, apperr.UnsupportedFormat(extOf(filename))
	}

	processed, err := imgproc.Process(data, opts.Format, opts.MaxWidth)
	if err != nil {
		if errors.Is(err, imgproc.ErrUnsupportedFormat) {
			return Result{}, apperr.UnsupportedFormat(opts.Format)
		}
		return Result{}, apperr.ProcessError()
	}

	key, err := newKey()
	if err != nil {
		return Result{}, apperr.StorageError(err)
	}

	if err := s.st.Put(ctx, key, bytes.NewReader(processed.Data)); err != nil {
		return Result{}, apperr.StorageError(err)
	}

	img := meta.Image{
		Key:       key,
		Format:    processed.Format,
		Size:      int64(len(processed.Data)),
		Width:     processed.Width,
		Height:    processed.Height,
		CreatedAt: time.Now(),
	}
	if err := s.db.Insert(ctx, img); err != nil {
		// 元数据写入失败时回滚文件，避免产生不可见的孤儿文件。
		if derr := s.st.Delete(context.Background(), key); derr != nil {
			return Result{}, apperr.StorageError(fmt.Errorf("%v；回滚文件也失败: %v", err, derr))
		}
		return Result{}, apperr.StorageError(err)
	}

	return Result{Key: key, Format: img.Format, Size: img.Size, Width: img.Width, Height: img.Height}, nil
}

// UploadPath 读取本地路径并上传（MCP 直传，PRD 3.1 / 3.5）。
func (s *Service) UploadPath(ctx context.Context, path string, opts Options) (Result, error) {
	if err := s.checkPath(path); err != nil {
		return Result{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, apperr.FileNotFound(path)
		}
		return Result{}, apperr.StorageError(err)
	}
	defer func() { _ = f.Close() }()
	return s.Upload(ctx, f, filepath.Base(path), opts)
}

// UploadURL 下载远程图片并上传（MCP 的 url 来源，PRD 3.1）。
func (s *Service) UploadURL(ctx context.Context, rawURL string, opts Options) (Result, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, apperr.DownloadFailed(rawURL, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, apperr.DownloadFailed(rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Result{}, apperr.DownloadFailed(rawURL,
			fmt.Errorf("HTTP %d", resp.StatusCode))
	}
	if resp.ContentLength > s.cfg.MaxBytes {
		return Result{}, apperr.FileTooLarge(formatSize(resp.ContentLength))
	}
	return s.Upload(ctx, resp.Body, path.Base(mustPath(rawURL)), opts)
}

// mustPath 提取 URL 路径部分用于错误提示，失败时返回空串。
func mustPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

// GetMeta 只返回图片元数据（直连模式下预览页与 302 用，不触碰对象存储）。
func (s *Service) GetMeta(ctx context.Context, key string) (meta.Image, error) {
	return s.db.Get(ctx, key)
}

// Get 返回图片元数据与内容流。key 不存在时返回 meta.ErrNotFound。
func (s *Service) Get(ctx context.Context, key string) (meta.Image, io.ReadCloser, error) {
	img, err := s.db.Get(ctx, key)
	if err != nil {
		return meta.Image{}, nil, err
	}
	rc, err := s.st.Get(ctx, key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return meta.Image{}, nil, meta.ErrNotFound
		}
		return meta.Image{}, nil, apperr.StorageError(err)
	}
	return img, rc, nil
}

// ListImages 返回最近上传的图片元数据（画廊页用）。limit 越界时收敛到默认值。
func (s *Service) ListImages(ctx context.Context, limit int) ([]meta.Image, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.db.List(ctx, limit)
}

// DeleteImage 删除图片：先删元数据（立即不可见），再删文件。
// 文件删除失败仅记日志并登记孤儿文件（DEV_LOG），不回滚 DB —— 宁可留孤儿文件，
// 也不让元数据指向已消失的文件造成裂图。
func (s *Service) DeleteImage(ctx context.Context, key string) error {
	if err := s.db.Delete(ctx, key); err != nil {
		return err
	}
	if err := s.st.Delete(ctx, key); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("孤儿文件待清理：删除图片文件失败 key=%s: %v", key, err)
	}
	return nil
}

// checkPath 校验本地路径是否在白名单内（PRD 3.5）。
// 实现提示 1：必须 Abs+Clean 后做边界匹配，不能只用字符串前缀。
func (s *Service) checkPath(path string) error {
	if s.cfg.AllowAll {
		return nil
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		return apperr.PathNotAllowed(path, s.cfg.AllowedPaths)
	}
	clean = filepath.Clean(clean)

	for _, allowed := range s.cfg.AllowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		allowedClean := filepath.Clean(allowedAbs)
		if clean == allowedClean || strings.HasPrefix(clean, allowedClean+string(filepath.Separator)) {
			return nil
		}
	}
	return apperr.PathNotAllowed(path, s.cfg.AllowedPaths)
}

// newKey 生成 8 位随机 key（base62，crypto/rand）。
func newKey() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机 key 失败: %w", err)
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// extOf 取文件扩展名（含点），无扩展名时返回空串。
func extOf(name string) string {
	return filepath.Ext(name)
}

// formatSize 把字节数格式化为人类可读大小（如 "15MB"）。
func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	value := float64(n) / float64(div)
	units := []string{"KB", "MB", "GB"}
	s := fmt.Sprintf("%.1f", value)
	if strings.HasSuffix(s, ".0") {
		s = strings.TrimSuffix(s, ".0")
	}
	return s + units[exp]
}
