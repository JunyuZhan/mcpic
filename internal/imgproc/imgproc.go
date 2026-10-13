// Package imgproc 负责图片识别与处理，与 HTTP 层解耦（宪法 8.1）。
// D1 范围：魔数识别 + 尺寸读取。格式转换与缩放属于 D2。
package imgproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // 注册 GIF 解码器
	_ "image/jpeg" // 注册 JPEG 解码器
	_ "image/png"  // 注册 PNG 解码器

	_ "github.com/deepteams/webp" // 纯 Go WebP（init 时向 image 注册解码器）
)

// ErrUnsupportedFormat 表示文件不是受支持的图片格式。
var ErrUnsupportedFormat = errors.New("不支持的图片格式")

// ErrCorrupt 表示图片数据无法解码。
var ErrCorrupt = errors.New("图片可能已损坏")

// SupportedFormats 是 PRD 错误码表中声明支持的格式。
const SupportedFormats = "jpg/png/webp/gif"

// Info 是图片头部探测结果。
type Info struct {
	// Format 为规范化格式名：png / jpg / webp / gif。
	Format string
	Width  int
	Height int
}

// Detect 用魔数（而非后缀名，实现提示 2）识别图片格式。
// 无法识别时返回 ErrUnsupportedFormat。
func Detect(b []byte) (string, error) {
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png", nil
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpg", nil
	case len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp", nil
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return "gif", nil
	default:
		return "", ErrUnsupportedFormat
	}
}

// Probe 读取图片头部得到尺寸与格式。数据损坏时返回 ErrCorrupt。
func Probe(b []byte) (Info, error) {
	if _, err := Detect(b); err != nil {
		return Info{}, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return Info{
		Format: normalizeFormat(format),
		Width:  cfg.Width,
		Height: cfg.Height,
	}, nil
}

func normalizeFormat(format string) string {
	if format == "jpeg" {
		return "jpg"
	}
	return format
}

// ContentType 返回格式对应的 MIME 类型。
func ContentType(format string) string {
	switch format {
	case "png":
		return "image/png"
	case "jpg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// Ext 返回格式对应的文件扩展名（不含点）。
func Ext(format string) string {
	return format
}
