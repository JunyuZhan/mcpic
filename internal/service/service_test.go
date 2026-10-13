package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcpic/mcpic/internal/apperr"
	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/storage"
)

func newTestService(t *testing.T) (*Service, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		StoragePath:  dir,
		AllowedPaths: []string{"/tmp", t.TempDir()},
		MaxBytes:     config.MaxUploadBytes,
	}
	st, err := storage.NewLocal(cfg.StoragePath)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	db, err := meta.Open(filepath.Join(cfg.StoragePath, "test.db"))
	if err != nil {
		t.Fatalf("meta.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(cfg, st, db), cfg
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestUploadSuccess(t *testing.T) {
	svc, _ := newTestService(t)
	res, err := svc.Upload(context.Background(), bytes.NewReader(pngBytes(t, 100, 100)), "a.png", Options{})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.Format != "webp" {
		t.Fatalf("默认应转 webp, got %s", res.Format)
	}
	if res.Width != 100 || res.Height != 100 {
		t.Fatalf("尺寸 = %dx%d", res.Width, res.Height)
	}
	if len(res.Key) != 8 {
		t.Fatalf("key 长度应为 8, got %q", res.Key)
	}
	// 元数据可读且文件可读。
	img, rc, err := svc.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if img.Format != "webp" || img.Size <= 0 {
		t.Fatalf("元数据 = %+v", img)
	}
}

func TestUploadUnsupportedFormat(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Upload(context.Background(), bytes.NewReader([]byte("not an image")), "malware.bmp", Options{})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeUnsupportedFormat {
		t.Fatalf("期望 UNSUPPORTED_FORMAT, got %v", err)
	}
	if ae.Message != "不支持 .bmp 格式，支持：jpg/png/webp/gif" {
		t.Fatalf("错误消息不符: %q", ae.Message)
	}
}

func TestUploadFileTooLarge(t *testing.T) {
	svc, cfg := newTestService(t)
	cfg.MaxBytes = 10
	_, err := svc.Upload(context.Background(), bytes.NewReader(pngBytes(t, 20, 20)), "big.png", Options{})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeFileTooLarge {
		t.Fatalf("期望 FILE_TOO_LARGE, got %v", err)
	}
}

func TestUploadPathNotAllowed(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.UploadPath(context.Background(), "/etc/passwd")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodePathNotAllowed {
		t.Fatalf("期望 PATH_NOT_ALLOWED, got %v", err)
	}
	if ae.Message != "路径 /etc/passwd 不在允许范围内，允许的路径前缀："+formatAllowed(svc.cfg.AllowedPaths) {
		t.Fatalf("错误消息不符: %q", ae.Message)
	}
}

func TestUploadPathPrefixBoundary(t *testing.T) {
	// /tmp 是白名单，但 /tmp_evil 不是（实现提示 1 的边界匹配）。
	svc, cfg := newTestService(t)
	cfg.AllowedPaths = []string{"/tmp"}
	svc.cfg = cfg

	_, err := svc.UploadPath(context.Background(), "/tmp_evil/x.png")
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		// 若文件不存在且路径判定为允许，会走到 FILE_NOT_FOUND —— 那说明边界匹配失败。
		t.Fatalf("路径 /tmp_evil 应被拒绝, got %v", err)
	}
	if ae.Code != apperr.CodePathNotAllowed {
		t.Fatalf("期望 PATH_NOT_ALLOWED, got %v", ae.Code)
	}
}

func TestUploadPathNotFound(t *testing.T) {
	svc, cfg := newTestService(t)
	tmp := t.TempDir()
	cfg.AllowedPaths = []string{tmp}
	svc.cfg = cfg

	_, err := svc.UploadPath(context.Background(), filepath.Join(tmp, "nope.png"))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeFileNotFound {
		t.Fatalf("期望 FILE_NOT_FOUND, got %v", err)
	}
	if ae.Message != "路径 "+filepath.Join(tmp, "nope.png")+" 不存在，请检查路径" {
		t.Fatalf("错误消息不符: %q", ae.Message)
	}
}

func TestUploadPathSucceeds(t *testing.T) {
	svc, cfg := newTestService(t)
	tmp := t.TempDir()
	cfg.AllowedPaths = []string{tmp}
	svc.cfg = cfg

	p := filepath.Join(tmp, "in.png")
	if err := os.WriteFile(p, pngBytes(t, 50, 50), 0o600); err != nil {
		t.Fatalf("写测试文件: %v", err)
	}
	res, err := svc.UploadPath(context.Background(), p)
	if err != nil {
		t.Fatalf("UploadPath: %v", err)
	}
	if res.Format != "webp" {
		t.Fatalf("format = %s", res.Format)
	}
}

func TestGetMissingKey(t *testing.T) {
	svc, _ := newTestService(t)
	if _, _, err := svc.Get(context.Background(), "missing1"); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("期望 meta.ErrNotFound, got %v", err)
	}
}

func TestAllowAllPaths(t *testing.T) {
	svc, cfg := newTestService(t)
	cfg.AllowAll = true
	svc.cfg = cfg
	// /etc/passwd 路径允许但会因格式失败 —— 只验证白名单放行。
	_, err := svc.UploadPath(context.Background(), "/etc/hostname")
	var ae *apperr.Error
	if errors.As(err, &ae) && ae.Code == apperr.CodePathNotAllowed {
		t.Fatalf("AllowAll 应放行路径: %v", err)
	}
}

func formatAllowed(paths []string) string {
	return "[" + joinComma(paths) + "]"
}

func joinComma(paths []string) string {
	out := ""
	for i, p := range paths {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}
