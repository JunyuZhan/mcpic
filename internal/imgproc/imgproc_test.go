package imgproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestDetectMagicBytes(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
		ok   bool
	}{
		{"png", makePNG(t, 4, 4), "png", true},
		{"jpg", makeJPEG(t, 4, 4), "jpg", true},
		{"gif", []byte("GIF89a...."), "gif", true},
		{"webp头", append([]byte("RIFF\x00\x00\x00\x00WEBP"), makePNG(t, 2, 2)...), "webp", true},
		{"伪造后缀的非图片", []byte("MZ\x90\x00 this is not an image"), "", false},
		{"空数据", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Detect(tc.data)
			if tc.ok {
				if err != nil || got != tc.want {
					t.Fatalf("Detect = (%q, %v), want %q", got, err, tc.want)
				}
			} else if !errors.Is(err, ErrUnsupportedFormat) {
				t.Fatalf("期望 ErrUnsupportedFormat, got %v", err)
			}
		})
	}
}

func TestProbeReadsDimensions(t *testing.T) {
	info, err := Probe(makePNG(t, 100, 50))
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if info.Width != 100 || info.Height != 50 || info.Format != "png" {
		t.Fatalf("Probe = %+v", info)
	}
}

func TestProcessPNGtoWebP(t *testing.T) {
	out, err := Process(makePNG(t, 100, 50), "webp", 1920)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out.Format != "webp" {
		t.Fatalf("format = %s, want webp", out.Format)
	}
	if _, err := Detect(out.Data); err != nil {
		t.Fatalf("输出不是合法 WebP: %v", err)
	}
	if out.Width != 100 || out.Height != 50 {
		t.Fatalf("尺寸 = %dx%d", out.Width, out.Height)
	}
}

func TestProcessResize(t *testing.T) {
	out, err := Process(makePNG(t, 3840, 2160), "original", 1920)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out.Width != 1920 {
		t.Fatalf("宽 = %d, want 1920", out.Width)
	}
	if out.Height != 1080 {
		t.Fatalf("高 = %d, want 1080", out.Height)
	}
	if out.Format != "png" {
		t.Fatalf("format = %s, want png (original 不转格式)", out.Format)
	}
}

func TestProcessOriginalKeepsFormat(t *testing.T) {
	src := makeJPEG(t, 100, 100)
	out, err := Process(src, "original", 1920)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out.Format != "jpg" || !bytes.Equal(out.Data, src) {
		t.Fatalf("original 未超宽时应原样返回, format=%s", out.Format)
	}
}

func TestProcessInvalidFormatParam(t *testing.T) {
	if _, err := Process(makePNG(t, 4, 4), "bmp", 1920); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("期望 ErrUnsupportedFormat, got %v", err)
	}
}

func TestProcessCorruptData(t *testing.T) {
	// PNG 魔数 + 损坏的后续数据。
	fake := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("garbage")...)
	if _, err := Process(fake, "webp", 1920); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("期望 ErrCorrupt, got %v", err)
	}
}

func TestProcessGIFPassthrough(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewPaletted(image.Rect(0, 0, 4000, 10), color.Palette{color.Black, color.White})
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif.Encode: %v", err)
	}
	out, err := Process(buf.Bytes(), "webp", 1920)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out.Format != "gif" || out.Width != 4000 {
		t.Fatalf("GIF 应原样存储, format=%s width=%d", out.Format, out.Width)
	}
}
