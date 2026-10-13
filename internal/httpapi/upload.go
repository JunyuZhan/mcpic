package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mcpic/mcpic/internal/apperr"
	"github.com/mcpic/mcpic/internal/imgproc"
	"github.com/mcpic/mcpic/internal/service"
)

// uploadResponse 是 PRD 3.2 的成功响应。
type uploadResponse struct {
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
	Size     int64  `json:"size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
}

// uploadJSON 是 JSON 上传请求体。
type uploadJSON struct {
	Data     string `json:"data"`                // base64 编码的图片数据
	Format   string `json:"format,omitempty"`    // webp（默认）/ original
	MaxWidth int    `json:"max_width,omitempty"` // 默认 1920
}

// handleUpload 实现 POST /api/upload，接受 multipart 或 JSON（PRD 3.2）。
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 硬上限留给 service 层做精确大小判定（multipart 用 FileHeader.Size、
	// JSON 用解码后长度），这里只挡极端滥用。
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBytes*10+4096)

	opts, reader, filename, err := s.parseUpload(r)
	if err != nil {
		writeAppError(w, err)
		return
	}

	result, err := s.svc.Upload(r.Context(), reader, filename, opts)
	if err != nil {
		writeAppError(w, err)
		return
	}

	base := getBaseURL(s.cfg, r)
	url := base + s.svc.URL(result.Key)
	writeJSON(w, http.StatusOK, uploadResponse{
		URL:      url,
		Markdown: "![image](" + url + ")",
		Size:     result.Size,
		Width:    result.Width,
		Height:   result.Height,
		Format:   result.Format,
	})
}

// parseUpload 解析请求体为 (处理参数, 图片数据流, 文件名)。
func (s *Server) parseUpload(r *http.Request) (service.Options, io.Reader, string, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))

	switch mediaType {
	case "multipart/form-data":
		return s.parseMultipart(r)
	case "application/json":
		return s.parseJSON(r)
	default:
		return service.Options{}, nil, "", apperr.New("BAD_CONTENT_TYPE",
			"不支持的 Content-Type："+mediaType+"，请使用 multipart/form-data 或 application/json")
	}
}

func (s *Server) parseMultipart(r *http.Request) (service.Options, io.Reader, string, error) {
	if err := r.ParseMultipartForm(s.cfg.MaxBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return service.Options{}, nil, "", apperr.FileTooLarge(formatSize(s.cfg.MaxBytes * 10))
		}
		return service.Options{}, nil, "", apperr.New("BAD_REQUEST",
			"解析 multipart 失败："+err.Error()+"，请检查字段名是否为 file")
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return service.Options{}, nil, "", apperr.New("BAD_REQUEST",
			"缺少文件字段：请使用 -F \"file=@图片路径\" 上传")
	}
	// 用真实文件大小做精确判定，避免截断值出现在错误消息里。
	if header.Size > s.cfg.MaxBytes {
		_ = file.Close()
		return service.Options{}, nil, "", apperr.FileTooLarge(formatSize(header.Size))
	}

	opts := service.Options{
		Format:   firstNonEmpty(r.FormValue("format"), "webp"),
		MaxWidth: parseMaxWidth(r.FormValue("max_width")),
	}
	if err := validateOptions(opts); err != nil {
		_ = file.Close()
		return service.Options{}, nil, "", err
	}
	return opts, file, header.Filename, nil
}

func (s *Server) parseJSON(r *http.Request) (service.Options, io.Reader, string, error) {
	var body uploadJSON
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		return service.Options{}, nil, "", apperr.New("BAD_REQUEST",
			"解析 JSON 失败："+err.Error()+"，请提供 {\"data\": \"<base64>\"}")
	}
	if body.Data == "" {
		return service.Options{}, nil, "", apperr.New("BAD_REQUEST",
			"缺少 data 字段：JSON 上传需要 {\"data\": \"<base64 编码的图片>\"}")
	}
	raw, err := base64.StdEncoding.DecodeString(body.Data)
	if err != nil {
		return service.Options{}, nil, "", apperr.New("BAD_REQUEST",
			"data 不是合法的 base64："+err.Error())
	}
	if int64(len(raw)) > s.cfg.MaxBytes {
		return service.Options{}, nil, "", apperr.FileTooLarge(formatSize(int64(len(raw))))
	}
	opts := service.Options{
		Format:   firstNonEmpty(body.Format, "webp"),
		MaxWidth: body.MaxWidth,
	}
	if opts.MaxWidth == 0 {
		opts.MaxWidth = imgproc.DefaultMaxWidth
	}
	if err := validateOptions(opts); err != nil {
		return service.Options{}, nil, "", err
	}
	return opts, strings.NewReader(string(raw)), "", nil
}

// validateOptions 校验 format / max_width 取值。
func validateOptions(opts service.Options) error {
	if opts.Format != "webp" && opts.Format != "original" {
		return apperr.UnsupportedFormat(opts.Format)
	}
	if opts.MaxWidth <= 0 {
		return apperr.New("BAD_REQUEST",
			"max_width 必须是正整数，例如 max_width=1920")
	}
	return nil
}

func parseMaxWidth(v string) int {
	if v == "" {
		return imgproc.DefaultMaxWidth
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return imgproc.DefaultMaxWidth
	}
	return n
}

// formatSize 把字节数格式化为人类可读大小（如 "12MB"）。
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
