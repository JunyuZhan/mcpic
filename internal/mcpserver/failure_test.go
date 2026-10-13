package mcpserver

import (
	"strings"
	"testing"

	"github.com/mcpic/mcpic/internal/apperr"
)

func TestPickSourceThreeWay(t *testing.T) {
	if _, err := pickSource(uploadArgs{}); err == nil {
		t.Fatal("空参数应报错")
	}
	if _, err := pickSource(uploadArgs{Path: "/a", URL: "http://b"}); err == nil {
		t.Fatal("多选应报错")
	}
	for _, tc := range []struct {
		args uploadArgs
		want source
	}{
		{uploadArgs{Path: "/a"}, sourcePath},
		{uploadArgs{URL: "http://a"}, sourceURL},
		{uploadArgs{Data: "AAAA"}, sourceData},
	} {
		got, err := pickSource(tc.args)
		if err != nil || got != tc.want {
			t.Fatalf("pickSource(%+v) = (%v, %v), want %v", tc.args, got, err, tc.want)
		}
	}
}

// TestRenderFailureThreeElements 校验错误文案包含三要素：
// 错误类型、相关输入、可执行的下一步（PRD 3.1 / 宪法第七条）。
func TestRenderFailureThreeElements(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		input string
		want  []string
	}{
		{
			name:  "A4 文件不存在",
			err:   apperr.FileNotFound("/tmp/screenshot.png"),
			input: "路径: /tmp/screenshot.png",
			want:  []string{"上传失败：文件不存在", "路径: /tmp/screenshot.png", "请检查路径是否正确。"},
		},
		{
			name:  "A10 路径白名单",
			err:   apperr.PathNotAllowed("/etc/passwd", []string{"/tmp", "/home", "/Users"}),
			input: "路径: /etc/passwd",
			want: []string{
				"上传失败：路径不在允许范围内",
				"路径: /etc/passwd",
				"MCPIC_ALLOWED_PATHS",
			},
		},
		{
			name:  "格式不支持",
			err:   apperr.UnsupportedFormat(".bmp"),
			input: "路径: /tmp/a.bmp",
			want:  []string{"上传失败：格式不支持", "jpg/png/webp/gif"},
		},
		{
			name:  "下载失败",
			err:   apperr.DownloadFailed("http://x/i.png", errStub{}),
			input: "URL: http://x/i.png",
			want:  []string{"上传失败：远程图片下载失败", "URL: http://x/i.png", "公网可达"},
		},
		{
			name:  "三选一参数错误",
			err:   apperr.BadRequest("必须提供 path、url、data 三者之一"),
			input: "参数: 未提供 path / url / data",
			want:  []string{"上传失败：参数错误", "参数: 未提供", "path / url / data"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderFailure(tc.err, tc.input)
			// 三要素：第一行类型、第二行输入、第三行下一步。
			lines := strings.Split(got, "\n")
			if len(lines) != 3 {
				t.Fatalf("应为 3 行，got %d: %q", len(lines), got)
			}
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("缺少要素 %q：\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderFailureUnknownError(t *testing.T) {
	got := renderFailure(errStub{}, "路径: /x")
	if strings.Contains(got, "internal error") || strings.Contains(got, "something went wrong") {
		t.Fatalf("禁止模糊错误: %q", got)
	}
	if !strings.Contains(got, "反馈给维护者") {
		t.Fatalf("未知错误应给出下一步: %q", got)
	}
}

type errStub struct{}

func (errStub) Error() string { return "stub" }
