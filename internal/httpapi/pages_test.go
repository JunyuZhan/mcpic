package httpapi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mcpic/mcpic/internal/config"
)

// TestUploadPage 验收（D4.5/D4.6/D4.7）：上传页可访问，含拖拽/多选/队列/重试与预压缩文案。
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
	for _, want := range []string{
		"拖拽图片到此处", "paste", "apikey", "/gallery",
		"multiple",      // D4.6 多选
		"queue",         // D4.6 串行队列
		"重试",            // D4.6 失败单独重试
		"上传成功",          // 全部完成状态
		"data-copy-src", // 结果项四格式复制（动态生成）
		"预压缩",           // D4.7 超过 3MB 前端预压缩
	} {
		if !strings.Contains(html, want) {
			t.Errorf("上传页缺少 %q", want)
		}
	}
	if strings.Contains(html, "单张最大 10MB") && !strings.Contains(html, "预压缩") {
		t.Errorf("10MB 文案必须搭配预压缩说明")
	}
}

// TestHomeDynamicBaseURL 验收（D4.7 P3）：首页 MCP 配置示例的 BASE_URL
// 必须来自站点配置（或请求推断），绝不能硬编码 mcpic.cn。
func TestHomeDynamicBaseURL(t *testing.T) {
	// 未配置 BaseURL：回退请求 Host。
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	html := string(body)
	if strings.Contains(html, "mcpic.cn") {
		t.Fatalf("首页硬编码了 mcpic.cn")
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	if !strings.Contains(html, host) {
		t.Fatalf("首页 MCP 配置示例应包含请求推断的 host %q", host)
	}

	// 显式配置 BaseURL：示例必须用配置值。
	srv2, _ := newTestServer(t, func(c *config.Config) { c.BaseURL = "https://pic.example.com" })
	resp2, err := http.Get(srv2.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	b2, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if !strings.Contains(string(b2), "https://pic.example.com") {
		t.Fatalf("配置 BaseURL 后首页示例应为配置值")
	}
	if strings.Contains(string(b2), "mcpic.cn") {
		t.Fatalf("配置后仍出现 mcpic.cn")
	}
}

// TestGalleryShellAndAPIAuth 验收（D4.7 P4）：/gallery 仅返回无数据壳页；
// 图片列表 API /api/gallery 必须鉴权。
func TestGalleryShellAndAPIAuth(t *testing.T) {
	srv, _ := newTestServer(t, func(c *config.Config) { c.APIKey = "secret" })

	// 壳页：200、无图片数据、含客户端门禁与 API 地址。
	resp, err := http.Get(srv.URL + "/gallery")
	if err != nil {
		t.Fatalf("GET /gallery: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gallery status = %d", resp.StatusCode)
	}
	html := string(body)
	for _, want := range []string{"localStorage", "/api/gallery", "/upload"} {
		if !strings.Contains(html, want) {
			t.Errorf("画廊壳页缺少 %q", want)
		}
	}

	// API 无 key → 401。
	r1, _ := http.Get(srv.URL + "/api/gallery")
	_ = r1.Body.Close()
	if r1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("无 key 应 401, got %d", r1.StatusCode)
	}

	// API 错 key → 401。
	rq2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/gallery", nil)
	rq2.Header.Set("X-API-Key", "wrong")
	r2, err := http.DefaultClient.Do(rq2)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("错 key 应 401, got %d", r2.StatusCode)
	}

	// API 正确 key → 200 JSON。
	rq3, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/gallery", nil)
	rq3.Header.Set("X-API-Key", "secret")
	r3, err := http.DefaultClient.Do(rq3)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = r3.Body.Close() }()
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("正确 key 应 200, got %d", r3.StatusCode)
	}
	var list struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := jsonDecode(r3, &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// TestGalleryUploadDeleteFlow 验收（D4.5/D4.7）：上传 → API 列表出现 →
// 无 key 401 → 有 key 删除成功 → 列表与直链均消失 → 重复删除 404。
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
		t.Fatalf("无法从 URL 提取 key: %q", up.URL)
	}

	// 壳页不含任何图片数据。
	page, _ := http.Get(srv.URL + "/gallery")
	pb, _ := io.ReadAll(page.Body)
	_ = page.Body.Close()
	if strings.Contains(string(pb), key) {
		t.Fatalf("壳页泄露了图片 key")
	}

	// API 列表出现该图与四格式内容。
	gal, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/gallery", nil)
	gal.Header.Set("X-API-Key", "secret")
	gr, err := http.DefaultClient.Do(gal)
	if err != nil {
		t.Fatalf("GET /api/gallery: %v", err)
	}
	gb, _ := io.ReadAll(gr.Body)
	_ = gr.Body.Close()
	if gr.StatusCode != http.StatusOK {
		t.Fatalf("api/gallery status = %d", gr.StatusCode)
	}
	for _, want := range []string{key, "markdown", "bbcode"} {
		if !strings.Contains(string(gb), want) {
			t.Errorf("API 列表缺少 %q", want)
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

	// API 列表不再包含。
	gal2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/gallery", nil)
	gal2.Header.Set("X-API-Key", "secret")
	gr2, err := http.DefaultClient.Do(gal2)
	if err != nil {
		t.Fatalf("GET /api/gallery: %v", err)
	}
	gb2, _ := io.ReadAll(gr2.Body)
	_ = gr2.Body.Close()
	if strings.Contains(string(gb2), key) {
		t.Errorf("删除后列表仍包含 %q", key)
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
