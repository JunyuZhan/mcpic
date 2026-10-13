package imgproc

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	webp "github.com/deepteams/webp" // 纯 Go WebP 编解码（已获人类批准）
	"golang.org/x/image/draw"
)

// WebPQuality 是 PRD 第 5 节固定的 WebP 质量。
const WebPQuality = 85

// DefaultMaxWidth 是 PRD 3.1 的默认最大宽度。
const DefaultMaxWidth = 1920

// Processed 是处理结果。
type Processed struct {
	Data   []byte
	Format string
	Width  int
	Height int
}

// Process 执行 PRD 第 5 节的固定处理流程：
//
//	上传 → 如果是 PNG/JPEG → 转 WebP（质量 85）
//	     → 如果宽度 > maxWidth → 等比缩放到 maxWidth
//	     → 存储
//
// targetFormat 取值：webp（默认）/ original。
// GIF 与动画 WebP 一律原样存储：重编码只保留首帧会造成不可逆的数据丢失，
// 这是 PRD 未覆盖的边界，按实现提示 5 的降级精神处理（保成功，不保处理）。
func Process(data []byte, targetFormat string, maxWidth int) (Processed, error) {
	if maxWidth <= 0 {
		maxWidth = DefaultMaxWidth
	}
	if targetFormat == "" {
		targetFormat = "webp"
	}
	if targetFormat != "webp" && targetFormat != "original" {
		return Processed{}, fmt.Errorf("%w: format=%s（仅支持 webp / original）", ErrUnsupportedFormat, targetFormat)
	}

	srcFormat, err := Detect(data)
	if err != nil {
		return Processed{}, err
	}

	probe, err := Probe(data)
	if err != nil {
		return Processed{}, err
	}

	convertsToWebP := targetFormat == "webp" && (srcFormat == "png" || srcFormat == "jpg")
	needsResize := probe.Width > maxWidth

	if !convertsToWebP && !needsResize {
		// 无需处理：原样存储。
		return Processed{Data: data, Format: probe.Format, Width: probe.Width, Height: probe.Height}, nil
	}

	if srcFormat == "gif" {
		return Processed{Data: data, Format: probe.Format, Width: probe.Width, Height: probe.Height}, nil
	}
	if animated, err := isAnimatedWebP(srcFormat, data); err == nil && animated {
		return Processed{Data: data, Format: probe.Format, Width: probe.Width, Height: probe.Height}, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Processed{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}

	if needsResize {
		img = scaleToWidth(img, maxWidth)
	}

	// 输出容器：默认保持源格式，PNG/JPEG 按 PRD 转 WebP。
	outFormat := srcFormat
	if convertsToWebP {
		outFormat = "webp"
	}

	var buf bytes.Buffer
	switch outFormat {
	case "webp":
		if err := webp.Encode(&buf, img, &webp.Options{Quality: WebPQuality}); err != nil {
			return Processed{}, fmt.Errorf("%w: webp 编码失败: %v", ErrCorrupt, err)
		}
	case "jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: WebPQuality}); err != nil {
			return Processed{}, fmt.Errorf("%w: jpeg 编码失败: %v", ErrCorrupt, err)
		}
	default:
		if err := png.Encode(&buf, img); err != nil {
			return Processed{}, fmt.Errorf("%w: png 编码失败: %v", ErrCorrupt, err)
		}
	}

	b := img.Bounds()
	return Processed{
		Data:   buf.Bytes(),
		Format: outFormat,
		Width:  b.Dx(),
		Height: b.Dy(),
	}, nil
}

// scaleToWidth 等比缩放到目标宽度（仅当图片更宽时）。
func scaleToWidth(src image.Image, maxWidth int) image.Image {
	b := src.Bounds()
	if b.Dx() <= maxWidth {
		return src
	}
	dstH := b.Dy() * maxWidth / b.Dx()
	if dstH < 1 {
		dstH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxWidth, dstH))
	// CatmullRom 缩图质量优于 ApproxBiLinear。
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// isAnimatedWebP 判断 WebP 是否为动图。
func isAnimatedWebP(srcFormat string, data []byte) (bool, error) {
	if srcFormat != "webp" {
		return false, nil
	}
	features, err := webp.GetFeatures(bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	return features.HasAnimation, nil
}
