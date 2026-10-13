package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/service"
	"github.com/mcpic/mcpic/internal/storage"
)

func newTestServer(t *testing.T, mutate ...func(*config.Config)) (*httptest.Server, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		StoragePath:  t.TempDir(),
		AllowedPaths: []string{t.TempDir()},
		MaxBytes:     config.MaxUploadBytes,
	}
	for _, m := range mutate {
		m(cfg)
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

	svc := service.New(cfg, st, db)
	srv := httptest.NewServer(New(cfg, svc).Routes())
	t.Cleanup(srv.Close)
	return srv, cfg
}

func pngPayload(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 10, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func multipartBody(t *testing.T, field, filename string, data []byte, form map[string]string) (*bytes.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range form {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_ = w.Close()
	return bytes.NewReader(buf.Bytes()), w.FormDataContentType()
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestUploadMultipartAndFetch(t *testing.T) {
	srv, _ := newTestServer(t)
	body, ctype := multipartBody(t, "file", "shot.png", pngPayload(t, 800, 600), nil)

	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST upload: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body=%s", resp.StatusCode, b)
	}

	var up struct {
		URL      string `json:"url"`
		Markdown string `json:"markdown"`
		Size     int64  `json:"size"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Format   string `json:"format"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&up); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if up.Format != "webp" || up.Width != 800 || up.Height != 600 {
		t.Fatalf("响应 = %+v", up)
	}
	if up.Markdown != "![image]("+up.URL+")" {
		t.Fatalf("markdown = %q", up.Markdown)
	}

	// A5：URL 可直接访问。
	fetch, err := http.Get(up.URL)
	if err != nil {
		t.Fatalf("GET image: %v", err)
	}
	defer func() { _ = fetch.Body.Close() }()
	if fetch.StatusCode != http.StatusOK {
		t.Fatalf("图片状态码 = %d", fetch.StatusCode)
	}
	if ct := fetch.Header.Get("Content-Type"); ct != "image/webp" {
		t.Fatalf("Content-Type = %q", ct)
	}
	got, _ := io.ReadAll(fetch.Body)
	if len(got) == 0 || !bytes.Equal(got[:4], []byte("RIFF")) {
		t.Fatalf("内容不是 WebP: %v", got[:min(8, len(got))])
	}
}

func TestUploadJSONBase64(t *testing.T) {
	srv, _ := newTestServer(t)
	payload := base64.StdEncoding.EncodeToString(pngPayload(t, 10, 10))
	resp, err := http.Post(srv.URL+"/api/upload", "application/json",
		bytes.NewReader([]byte(`{"data":"`+payload+`","format":"original"}`)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, b)
	}
	var up struct {
		Format string `json:"format"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&up)
	if up.Format != "png" {
		t.Fatalf("format = %s, want png", up.Format)
	}
}

func TestUploadRequiresAPIKey(t *testing.T) {
	srv, _ := newTestServer(t, func(c *config.Config) { c.APIKey = "secret" })

	body, ctype := multipartBody(t, "file", "a.png", pngPayload(t, 4, 4), nil)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/upload", body)
	req.Header.Set("Content-Type", ctype)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 key 应 401, got %d", resp.StatusCode)
	}
	var er struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&er)
	if er.Error != "UNAUTHORIZED" || er.Message != "API Key 无效或缺失" {
		t.Fatalf("错误体 = %+v", er)
	}

	// 带正确 key 应成功。
	body2, ctype2 := multipartBody(t, "file", "a.png", pngPayload(t, 4, 4), nil)
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/upload", body2)
	req2.Header.Set("Content-Type", ctype2)
	req2.Header.Set("X-API-Key", "secret")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("do2: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("带 key 应 200, got %d", resp2.StatusCode)
	}
}

func TestUploadBaseURLFallsBackToHost(t *testing.T) {
	// A9：不设 MCPIC_BASE_URL 时，返回的链接使用请求的 Host。
	srv, _ := newTestServer(t)
	body, ctype := multipartBody(t, "file", "a.png", pngPayload(t, 4, 4), nil)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/upload", body)
	req.Header.Set("Content-Type", ctype)
	req.Host = "test.local"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var up struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&up)
	if len(up.URL) < len("http://test.local") || up.URL[:len("http://test.local")] != "http://test.local" {
		t.Fatalf("url = %q, 应以 http://test.local 开头", up.URL)
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	r.Header.Set("X-Forwarded-Host", "proxy.example")
	r.Host = "direct.example" // Host 是字段，不走 Header map

	// 2. X-Forwarded-Host 优先于 Host。
	if got := getBaseURL(&config.Config{}, r); got != "https://proxy.example" {
		t.Fatalf("got %q", got)
	}
	// 3. 无转发头时用 Host。
	r.Header.Del("X-Forwarded-Host")
	if got := getBaseURL(&config.Config{}, r); got != "http://direct.example" {
		t.Fatalf("got %q", got)
	}
	// 1. 显式配置无条件优先。
	cfg := &config.Config{BaseURL: "https://mcpic.cn"}
	r.Header.Set("X-Forwarded-Host", "proxy.example")
	if got := getBaseURL(cfg, r); got != "https://mcpic.cn" {
		t.Fatalf("got %q", got)
	}
	// 4. 全空兜底。
	r.Host = ""
	r.Header.Del("X-Forwarded-Host")
	if got := getBaseURL(&config.Config{}, r); got != "http://localhost:8080" {
		t.Fatalf("got %q", got)
	}
}

func TestFetchMissingImage(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/i/notexist1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var er struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&er)
	if er.Error != "IMAGE_NOT_FOUND" {
		t.Fatalf("error = %s", er.Error)
	}
}

func TestUploadRejectsNonImage(t *testing.T) {
	srv, _ := newTestServer(t)
	body, ctype := multipartBody(t, "file", "evil.png", []byte("MZ\x90\x00fake exe"), nil)
	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var er struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&er)
	if er.Error != "UNSUPPORTED_FORMAT" {
		t.Fatalf("error = %s", er.Error)
	}
	if er.Message != "不支持 .png 格式，支持：jpg/png/webp/gif" {
		t.Fatalf("message = %q", er.Message)
	}
}
