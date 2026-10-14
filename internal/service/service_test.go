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
	_, err := svc.UploadPath(context.Background(), "/etc/passwd", Options{})
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

	_, err := svc.UploadPath(context.Background(), "/tmp_evil/x.png", Options{})
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

	_, err := svc.UploadPath(context.Background(), filepath.Join(tmp, "nope.png"), Options{})
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
	res, err := svc.UploadPath(context.Background(), p, Options{})
	if err != nil {
		t.Fatalf("UploadPath: %v", err)
	}
	if res.Format != "webp" {
		t.Fatalf("format = %s", res.Format)
	}
}

// TestUploadPathRespectsOptions 守护 BUG-2：path 来源必须应用 format / max_width。
func TestUploadPathRespectsOptions(t *testing.T) {
	svc, cfg := newTestService(t)
	tmp := t.TempDir()
	cfg.AllowedPaths = []string{tmp}
	svc.cfg = cfg

	p := filepath.Join(tmp, "wide.png")
	if err := os.WriteFile(p, pngBytes(t, 2400, 1600), 0o600); err != nil {
		t.Fatalf("写测试文件: %v", err)
	}

	t.Run("max_width 生效", func(t *testing.T) {
		res, err := svc.UploadPath(context.Background(), p, Options{MaxWidth: 400})
		if err != nil {
			t.Fatalf("UploadPath: %v", err)
		}
		if res.Width != 400 || res.Height != 266 {
			t.Fatalf("期望缩放到 400x266, got %dx%d", res.Width, res.Height)
		}
		if res.Format != "webp" {
			t.Fatalf("format = %s", res.Format)
		}
	})

	t.Run("format=original 生效", func(t *testing.T) {
		res, err := svc.UploadPath(context.Background(), p, Options{Format: "original"})
		if err != nil {
			t.Fatalf("UploadPath: %v", err)
		}
		if res.Format != "png" {
			t.Fatalf("original 应保留 png, got %s", res.Format)
		}
	})
}

func TestGetMissingKey(t *testing.T) {
	svc, _ := newTestService(t)
	if _, _, err := svc.Get(context.Background(), "missing1"); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("期望 meta.ErrNotFound, got %v", err)
	}
}

func TestGetMetaOnly(t *testing.T) {
	svc, _ := newTestService(t)
	res, err := svc.Upload(context.Background(), bytes.NewReader(pngBytes(t, 64, 64)), "a.png", Options{})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	img, err := svc.GetMeta(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if img.Key != res.Key || img.Format != res.Format {
		t.Fatalf("GetMeta 与 Upload 结果不一致: %+v vs %+v", img, res)
	}
	if _, err := svc.GetMeta(context.Background(), "missing-meta"); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("期望 meta.ErrNotFound, got %v", err)
	}
}

func TestAllowAllPaths(t *testing.T) {
	svc, cfg := newTestService(t)
	cfg.AllowAll = true
	svc.cfg = cfg
	// /etc/passwd 路径允许但会因格式失败 —— 只验证白名单放行。
	_, err := svc.UploadPath(context.Background(), "/etc/hostname", Options{})
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

// uploadOne 上传一张测试图并返回结果。
func uploadOne(t *testing.T, svc *Service) Result {
	t.Helper()
	res, err := svc.Upload(context.Background(), bytes.NewReader(pngBytes(t, 40, 30)), "t.png", Options{})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	return res
}

// TestListImagesOrderAndLimit 守护画廊列表：倒序、limit 生效、越界收敛。
func TestListImagesOrderAndLimit(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	var keys []string
	for i := 0; i < 3; i++ {
		keys = append(keys, uploadOne(t, svc).Key)
	}

	list, err := svc.ListImages(ctx, 10)
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("条数 = %d", len(list))
	}
	if list[0].Key != keys[2] || list[2].Key != keys[0] {
		t.Fatalf("应按创建时间倒序, got [%s %s %s]", list[0].Key, list[1].Key, list[2].Key)
	}

	one, err := svc.ListImages(ctx, 1)
	if err != nil || len(one) != 1 || one[0].Key != keys[2] {
		t.Fatalf("limit=1 结果: %v err=%v", one, err)
	}

	// limit 越界收敛到默认 100，不报错。
	def, err := svc.ListImages(ctx, 0)
	if err != nil || len(def) != 3 {
		t.Fatalf("limit=0 应收敛默认: len=%d err=%v", len(def), err)
	}
}

// TestDeleteImage 守护删除闭环：先 DB 后文件，重复删除报 ErrNotFound。
func TestDeleteImage(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	res := uploadOne(t, svc)

	if err := svc.DeleteImage(ctx, res.Key); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if _, _, err := svc.Get(ctx, res.Key); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("删除后应 ErrNotFound, got %v", err)
	}
	if _, err := svc.st.Get(ctx, res.Key); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除后文件应不存在, got %v", err)
	}
	if err := svc.DeleteImage(ctx, res.Key); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("重复删除应 ErrNotFound, got %v", err)
	}
	if err := svc.DeleteImage(ctx, "missing00"); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("不存在 key 应 ErrNotFound, got %v", err)
	}
}

// failDeleteStorage 删除文件时返回硬错误（非 ErrNotExist），用于孤儿文件路径。
type failDeleteStorage struct {
	storage.Storage
}

func (f *failDeleteStorage) Delete(context.Context, string) error {
	return errors.New("磁盘 I/O 错误")
}

// TestDeleteImageOrphanFileTolerated 守护「先删 DB 再删文件」：文件删除失败
// 只记孤儿文件日志，不回滚、不报错（元数据已删，页面不会裂图）。
func TestDeleteImageOrphanFileTolerated(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	res := uploadOne(t, svc)

	svc.st = &failDeleteStorage{Storage: svc.st}
	if err := svc.DeleteImage(ctx, res.Key); err != nil {
		t.Fatalf("文件删除失败不应向上报错: %v", err)
	}
	if _, _, err := svc.Get(ctx, res.Key); !errors.Is(err, meta.ErrNotFound) {
		t.Fatalf("元数据应已删除, got %v", err)
	}
}
