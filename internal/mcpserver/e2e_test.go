package mcpserver

import (
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/httpapi"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/service"
	"github.com/mcpic/mcpic/internal/storage"
)

var binary string

// TestMain 构建真实二进制，e2e 通过 stdio 子进程跑完整 MCP 协议（等价 Claude Code 接入路径）。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mcpic-bin")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "mcpic")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/mcpic")
	if out, err := build.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// e2eEnv 是一次闭环测试的公共设施：同一存储目录上的 HTTP 服务 + MCP 子进程。
type e2eEnv struct {
	t        *testing.T
	storeDir string
	imgDir   string
	http     *httptest.Server
	session  *mcp.ClientSession
	pngData  []byte
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	root := t.TempDir()
	storeDir := filepath.Join(root, "store")
	imgDir := filepath.Join(root, "imgs")
	for _, d := range []string{storeDir, imgDir} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	cfg := &config.Config{
		StoragePath:  storeDir,
		AllowedPaths: []string{imgDir, storeDir},
		MaxBytes:     config.MaxUploadBytes,
	}
	st, err := storage.NewLocal(storeDir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	db, err := meta.Open(filepath.Join(storeDir, "mcpic.db"))
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.New(cfg, st, db)

	// HTTP 服务与 MCP 共享同一存储目录（真实部署形态）。
	ts := httptest.NewServer(httpapi.New(cfg, svc).Routes())
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	cmd := exec.Command(binary, "mcp")
	cmd.Env = append(os.Environ(),
		"MCPIC_STORAGE_PATH="+storeDir,
		"MCPIC_BASE_URL="+ts.URL,
		"MCPIC_ALLOWED_PATHS="+imgDir+":"+storeDir,
	)
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("连接 MCP 子进程失败: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	env := &e2eEnv{t: t, storeDir: storeDir, imgDir: imgDir, http: ts, session: session}
	env.pngData = env.writePNG("sample.png", 640, 480)
	return env
}

func (e *e2eEnv) writePNG(name string, w, h int) []byte {
	e.t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 60, A: 255})
		}
	}
	p := filepath.Join(e.imgDir, name)
	f, err := os.Create(p)
	if err != nil {
		e.t.Fatalf("create: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		e.t.Fatalf("encode: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		e.t.Fatalf("read: %v", err)
	}
	return data
}

// call 执行 upload_image 并返回 (文案, isError)。
func (e *e2eEnv) call(args uploadArgs) (string, bool) {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	res, err := e.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "upload_image",
		Arguments: args,
	})
	if err != nil {
		e.t.Fatalf("CallTool 失败: %v", err)
	}
	if len(res.Content) == 0 {
		e.t.Fatal("工具返回空内容")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		e.t.Fatalf("内容类型 = %T", res.Content[0])
	}
	return tc.Text, res.IsError
}

// TestE2E_MCPUploadLoop 覆盖 A2（path 上传）、A5（URL 可访问）、A4、A10 与三种来源。
func TestE2E_MCPUploadLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e 需要构建二进制")
	}
	env := newE2EEnv(t)

	t.Run("A2 path 上传并返回链接", func(t *testing.T) {
		path := filepath.Join(env.imgDir, "sample.png")
		text, isErr := env.call(uploadArgs{Path: path})
		if isErr {
			t.Fatalf("不应失败: %s", text)
		}
		lines := strings.Split(text, "\n")
		if len(lines) != 3 || lines[0] != "上传成功" {
			t.Fatalf("文案格式不符: %q", text)
		}
		if !strings.HasPrefix(lines[1], "URL: "+env.http.URL+"/i/") {
			t.Fatalf("URL 不符: %q", lines[1])
		}
		url := strings.TrimPrefix(lines[1], "URL: ")
		if lines[2] != "Markdown: ![image]("+url+")" {
			t.Fatalf("Markdown 不符: %q", lines[2])
		}

		// A5：返回的链接可直接访问，且是转换后的 WebP。
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("HTTP %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/webp" {
			t.Fatalf("Content-Type = %s", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body[:4]) != "RIFF" {
			t.Fatalf("不是 WebP")
		}
	})

	t.Run("A4 不存在的路径", func(t *testing.T) {
		missing := filepath.Join(env.imgDir, "nope.png")
		text, isErr := env.call(uploadArgs{Path: missing})
		if !isErr {
			t.Fatalf("应返回工具错误: %s", text)
		}
		want := []string{"上传失败：文件不存在", "路径: " + missing, "请检查路径是否正确。"}
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("缺少 %q：\n%s", w, text)
			}
		}
	})

	t.Run("A10 路径白名单", func(t *testing.T) {
		text, isErr := env.call(uploadArgs{Path: "/etc/passwd"})
		if !isErr {
			t.Fatalf("应返回工具错误: %s", text)
		}
		want := []string{
			"上传失败：路径不在允许范围内",
			"路径: /etc/passwd",
			"MCPIC_ALLOWED_PATHS",
		}
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("缺少 %q：\n%s", w, text)
			}
		}
	})

	t.Run("data base64 上传", func(t *testing.T) {
		text, isErr := env.call(uploadArgs{Data: base64.StdEncoding.EncodeToString(env.pngData)})
		if isErr {
			t.Fatalf("不应失败: %s", text)
		}
		if !strings.HasPrefix(text, "上传成功\n") {
			t.Fatalf("文案: %q", text)
		}
	})

	t.Run("url 远程上传", func(t *testing.T) {
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(env.pngData)
		}))
		defer remote.Close()

		text, isErr := env.call(uploadArgs{URL: remote.URL + "/pic.png"})
		if isErr {
			t.Fatalf("不应失败: %s", text)
		}
		if !strings.HasPrefix(text, "上传成功\n") {
			t.Fatalf("文案: %q", text)
		}
	})

	t.Run("url 不可达给出明确错误", func(t *testing.T) {
		text, isErr := env.call(uploadArgs{URL: "http://127.0.0.1:1/none.png"})
		if !isErr {
			t.Fatalf("应失败: %s", text)
		}
		if !strings.Contains(text, "上传失败：远程图片下载失败") || !strings.Contains(text, "URL: http://127.0.0.1:1/none.png") {
			t.Fatalf("文案: %q", text)
		}
	})

	t.Run("三选一参数校验", func(t *testing.T) {
		text, isErr := env.call(uploadArgs{})
		if !isErr || !strings.Contains(text, "上传失败：参数错误") {
			t.Fatalf("文案: %q", text)
		}
		text, isErr = env.call(uploadArgs{Path: "/a", URL: "http://b"})
		if !isErr || !strings.Contains(text, "只能提供一个") {
			t.Fatalf("文案: %q", text)
		}
	})

	t.Run("format 参数生效", func(t *testing.T) {
		text, isErr := env.call(uploadArgs{
			Data:   base64.StdEncoding.EncodeToString(env.pngData),
			Format: "original",
		})
		if isErr {
			t.Fatalf("不应失败: %s", text)
		}
		url := strings.TrimPrefix(strings.Split(text, "\n")[1], "URL: ")
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
			t.Fatalf("format=original 应保持 png, got %s", ct)
		}
	})
}
