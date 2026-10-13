package httpapi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mcpic/mcpic/internal/config"
)

// TestUploadPage 验收（D4.5）：上传页可访问，含拖拽区、粘贴提示与 API Key 输入。
func TestUploadPage(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/upload")
	if err != nil {
		t.Fatalf("GET /upload: %v", err)
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
	for _, want := range []string{"拖拽图片到此处", "paste", "apikey", "v-md", "v-html", "v-bb", "/gallery"} {
		if !strings.Contains(html, want) {
			t.Errorf("上传页缺少 %q", want)
		}
	}
}

// TestGalleryEmptyState 验收（D4.5）：画廊无图时展示空状态与上传引导。
func TestGalleryEmptyState(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/gallery")
	if err != nil {
		t.Fatalf("GET /gallery: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, "还没有图片") || !strings.Contains(html, `href="/upload"`) {
		t.Fatalf("空状态缺失")
	}
	if strings.Contains(html, `class="grid"`) {
		t.Fatalf("空状态不应渲染网格")
	}
}

// TestGalleryUploadDeleteFlow 验收（D4.5）：上传 → 画廊出现 → 无 key 401 →
// 有 key 删除成功 → 画廊与直链均消失 → 重复删除 404。
func TestGalleryUploadDeleteFlow(t *testing.T) {
	srv, _ := newTestServer(t, func(c *config.Config) { c.APIKey = "secret" })

	// 上传一张图。
	body, ctype := multipartBody(t, "file", "g.png", pngPayload(t, 80, 60), nil)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/upload", body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("X-API-Key", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var up struct {
		URL string `json:"url"`
	}
	if err := jsonDecode(resp, &up); err != nil {
		t.Fatalf("decode: %v", err)
	}
	key := strings.TrimPrefix(up.URL, strings.TrimSuffix(srv.URL, "/")+"/i/")
	if key == "" {
		t.Fatalf("无法提取 key: %q", up.URL)
	}

	// 画廊出现该图与四格式按钮。
	gal, err := http.Get(srv.URL + "/gallery")
	if err != nil {
		t.Fatalf("GET /gallery: %v", err)
	}
	gb, _ := io.ReadAll(gal.Body)
	_ = gal.Body.Close()
	if gal.StatusCode != http.StatusOK {
		t.Fatalf("gallery status = %d", gal.StatusCode)
	}
	for _, want := range []string{key, "BBCode", "删除"} {
		if !strings.Contains(string(gb), want) {
			t.Errorf("画廊缺少 %q", want)
		}
	}

	// 无 key 删除 → 401。
	d1, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/images/"+key, nil)
	r1, err := http.DefaultClient.Do(d1)
	if err != nil {
		t.Fatalf("delete no-key: %v", err)
	}
	_ = r1.Body.Close()
	if r1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 key 删除应 401, got %d", r1.StatusCode)
	}

	// 错 key 删除 → 401。
	d2, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/images/"+key, nil)
	d2.Header.Set("X-API-Key", "wrong")
	r2, err := http.DefaultClient.Do(d2)
	if err != nil {
		t.Fatalf("delete bad-key: %v", err)
	}
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("错 key 删除应 401, got %d", r2.StatusCode)
	}

	// 正确 key 删除 → 200。
	d3, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/images/"+key, nil)
	d3.Header.Set("X-API-Key", "secret")
	r3, err := http.DefaultClient.Do(d3)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	var del struct {
		OK bool `json:"ok"`
	}
	err = jsonDecode(r3, &del)
	_ = r3.Body.Close()
	if r3.StatusCode != http.StatusOK || !del.OK {
		t.Fatalf("删除应 200 {ok:true}, got %d %+v", r3.StatusCode, del)
	}

	// 画廊不再包含。
	gal2, _ := http.Get(srv.URL + "/gallery")
	gb2, _ := io.ReadAll(gal2.Body)
	_ = gal2.Body.Close()
	if strings.Contains(string(gb2), key) {
		t.Errorf("删除后画廊仍包含 %q", key)
	}

	// 直链 404。
	img, _ := http.Get(srv.URL + "/i/" + key)
	_ = img.Body.Close()
	if img.StatusCode != http.StatusNotFound {
		t.Errorf("删除后直链应 404, got %d", img.StatusCode)
	}

	// 重复删除 → 404 IMAGE_NOT_FOUND。
	d4, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/images/"+key, nil)
	d4.Header.Set("X-API-Key", "secret")
	r4, err := http.DefaultClient.Do(d4)
	if err != nil {
		t.Fatalf("delete again: %v", err)
	}
	var er struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = jsonDecode(r4, &er)
	_ = r4.Body.Close()
	if r4.StatusCode != http.StatusNotFound {
		t.Fatalf("重复删除应 404, got %d", r4.StatusCode)
	}
	if er.Error != "IMAGE_NOT_FOUND" || er.Message == "" {
		t.Fatalf("错误体 = %+v（需错误码+三要素消息）", er)
	}
}

// TestHomeNavToPages 验收（D4.5）：首页导航包含上传与画廊入口。
func TestHomeNavToPages(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `href="/upload"`) || !strings.Contains(string(body), `href="/gallery"`) {
		t.Fatalf("首页导航缺少上传/画廊入口")
	}
}
