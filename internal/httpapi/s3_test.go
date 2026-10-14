package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/service"
	"github.com/mcpic/mcpic/internal/storage"
)

// fakeS3Server 起一个最小 S3 端点（path-style），供直连模式端到端测试。
func fakeS3Server(t *testing.T, bucket string) *httptest.Server {
	t.Helper()
	store := map[string][]byte{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] != bucket {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		key := parts[1]
		switch r.Method {
		case http.MethodPut:
			data, _ := io.ReadAll(r.Body)
			store[key] = data
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	}))
}

// newTestServerS3 构造直连模式（S3）的测试服务器，baseURL 指向 fake S3。
func newTestServerS3(t *testing.T) (*httptest.Server, *config.Config) {
	t.Helper()
	s3srv := fakeS3Server(t, "my-bucket")
	t.Cleanup(s3srv.Close)

	dir := t.TempDir()
	cfg := &config.Config{
		StoragePath:  dir,
		AllowedPaths: []string{dir},
		MaxBytes:     config.MaxUploadBytes,
		StorageType:  config.StorageS3,
		S3: config.S3Config{
			Endpoint:        s3srv.URL,
			Region:          "auto",
			Bucket:          "my-bucket",
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			PublicBaseURL:   "https://cdn.example.com",
			ForcePathStyle:  true,
		},
	}
	st, err := storage.NewS3(storage.S3Options{
		Endpoint:        cfg.S3.Endpoint,
		Region:          cfg.S3.Region,
		Bucket:          cfg.S3.Bucket,
		AccessKeyID:     cfg.S3.AccessKeyID,
		SecretAccessKey: cfg.S3.SecretAccessKey,
		PublicBaseURL:   cfg.S3.PublicBaseURL,
		ForcePathStyle:  cfg.S3.ForcePathStyle,
	})
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	db, err := meta.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("meta.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	svc := service.New(cfg, st, db)
	srv := httptest.NewServer(New(cfg, svc).Routes())
	t.Cleanup(srv.Close)
	return srv, cfg
}

func TestS3UploadReturnsDirectURL(t *testing.T) {
	srv, _ := newTestServerS3(t)
	body, ctype := multipartBody(t, "file", "a.png", pngPayload(t, 8, 8), nil)
	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, b)
	}
	var up struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&up)
	if !strings.HasPrefix(up.URL, "https://cdn.example.com/") {
		t.Fatalf("上传 url 应为直链, got %q", up.URL)
	}
}

func TestS3ImageRedirectsToDirectURL(t *testing.T) {
	srv, _ := newTestServerS3(t)
	body, ctype := multipartBody(t, "file", "a.png", pngPayload(t, 8, 8), nil)
	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var up struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&up)
	_ = resp.Body.Close()

	// 非浏览器请求 → 302 到直链。
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/i/"+keyOf(t, up.URL), nil)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	r2, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /i/key: %v", err)
	}
	defer func() { _ = r2.Body.Close() }()
	if r2.StatusCode != http.StatusFound {
		t.Fatalf("期望 302, got %d", r2.StatusCode)
	}
	if loc := r2.Header.Get("Location"); loc != up.URL {
		t.Fatalf("Location = %q, want %q", loc, up.URL)
	}
}

func TestS3ImagePreviewUsesDirectURL(t *testing.T) {
	srv, _ := newTestServerS3(t)
	body, ctype := multipartBody(t, "file", "a.png", pngPayload(t, 8, 8), nil)
	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var up struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&up)
	_ = resp.Body.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/i/"+keyOf(t, up.URL), nil)
	req.Header.Set("Accept", "text/html")
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET html: %v", err)
	}
	defer func() { _ = r2.Body.Close() }()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("预览页期望 200, got %d", r2.StatusCode)
	}
	html, _ := io.ReadAll(r2.Body)
	if !bytes.Contains(html, []byte(`src="https://cdn.example.com/`)) {
		t.Fatalf("预览页 img src 应为直链, got:\n%s", html)
	}
}

func keyOf(t *testing.T, url string) string {
	t.Helper()
	i := strings.LastIndex(url, "/")
	if i < 0 {
		t.Fatalf("url 无 key: %q", url)
	}
	return url[i+1:]
}
