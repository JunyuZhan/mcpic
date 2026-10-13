package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func jsonDecode(resp *http.Response, v any) error {
	return json.NewDecoder(resp.Body).Decode(v)
}

// TestHome 验收：首页可访问，含 MCP 配置示例与项目说明（D4）。
func TestHome(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, want := range []string{
		"upload_image",    // MCP 工具名
		"MCPIC_API_KEY",   // MCP env 示例
		"mcpServers",      // MCP 配置示例
		"curl -X POST",    // curl 上传示例
		"让 AI Agent 上传图片", // 项目说明标题
	} {
		if !strings.Contains(html, want) {
			t.Errorf("首页缺少 %q", want)
		}
	}
}

// TestImagePageHTML 验收：浏览器请求（Accept: text/html）返回预览页，
// 含图片、元数据与一键复制 Markdown。
func TestImagePageHTML(t *testing.T) {
	srv, _ := newTestServer(t)
	body, ctype := multipartBody(t, "file", "shot.png", pngPayload(t, 640, 480), nil)

	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST upload: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var up struct {
		URL string `json:"url"`
	}
	if err := jsonDecode(resp, &up); err != nil {
		t.Fatalf("decode: %v", err)
	}
	key := strings.TrimPrefix(up.URL, strings.TrimSuffix(srv.URL, "/")+"/i/")
	if key == "" {
		t.Fatalf("无法从 URL 提取 key: %q", up.URL)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/i/"+key, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,image/avif,image/webp,*/*;q=0.8")
	page, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET page: %v", err)
	}
	defer func() { _ = page.Body.Close() }()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", page.StatusCode)
	}
	if ct := page.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q，浏览器请求应返回页面", ct)
	}
	pb, _ := io.ReadAll(page.Body)
	html := string(pb)
	for _, want := range []string{"复制 Markdown", "?raw=1", "640×480", key} {
		if !strings.Contains(html, want) {
			t.Errorf("页面缺少 %q", want)
		}
	}

	// 页面引用的原图字节仍可取（?raw=1）。
	raw, err := http.Get(srv.URL + "/i/" + key + "?raw=1")
	if err != nil {
		t.Fatalf("GET raw: %v", err)
	}
	defer func() { _ = raw.Body.Close() }()
	if ct := raw.Header.Get("Content-Type"); ct != "image/webp" {
		t.Fatalf("raw Content-Type = %q", ct)
	}
	rb, _ := io.ReadAll(raw.Body)
	if len(rb) < 4 || string(rb[:4]) != "RIFF" {
		t.Fatalf("raw 不是 WebP")
	}
}

// TestImageBrowserAcceptStillRawForImages 验收：图片标签请求
// （Accept: image/*）必须拿到图片字节而非 HTML。
func TestImageBrowserAcceptStillRawForImages(t *testing.T) {
	srv, _ := newTestServer(t)
	body, ctype := multipartBody(t, "file", "shot.png", pngPayload(t, 32, 32), nil)

	resp, err := http.Post(srv.URL+"/api/upload", ctype, body)
	if err != nil {
		t.Fatalf("POST upload: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var up struct {
		URL string `json:"url"`
	}
	if err := jsonDecode(resp, &up); err != nil {
		t.Fatalf("decode: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, up.URL, nil)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	got, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = got.Body.Close() }()
	if ct := got.Header.Get("Content-Type"); ct != "image/webp" {
		t.Fatalf("Accept: image/* 时 Content-Type = %q，应返回图片", ct)
	}
}
