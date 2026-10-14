package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3Client 实现 s3Client 窄接口，用于单元测试。
type mockS3Client struct {
	store        map[string][]byte
	contentTypes map[string]string
	putErr       error
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{store: map[string][]byte{}, contentTypes: map[string]string{}}
}

func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	data, _ := io.ReadAll(params.Body)
	m.store[aws.ToString(params.Key)] = data
	m.contentTypes[aws.ToString(params.Key)] = aws.ToString(params.ContentType)
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	data, ok := m.store[aws.ToString(params.Key)]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(m.store, aws.ToString(params.Key))
	delete(m.contentTypes, aws.ToString(params.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) HeadObject(_ context.Context, params *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	data, ok := m.store[aws.ToString(params.Key)]
	if !ok {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(m.contentTypes[aws.ToString(params.Key)]),
	}, nil
}

// 构造 webp 魔数，供 DetectContentType 识别。
func webpMagic() []byte {
	return []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
}

// pngMagic 是最小可识别的 PNG 头（DetectContentType 只看前 8 字节）。
func pngMagic() []byte {
	return []byte("\x89PNG\r\n\x1a\n")
}

func TestS3URL(t *testing.T) {
	s := &S3{client: newMockS3Client(), bucket: "b", baseURL: "https://cdn.example.com"}
	if got := s.URL("abc123"); got != "https://cdn.example.com/abc123" {
		t.Fatalf("URL = %q", got)
	}
}

func TestS3PutSetsContentType(t *testing.T) {
	m := newMockS3Client()
	s := &S3{client: m, bucket: "b", baseURL: "https://cdn"}
	ctx := context.Background()

	if err := s.Put(ctx, "k1", bytes.NewReader(webpMagic())); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got := m.contentTypes["k1"]; got != "image/webp" {
		t.Fatalf("webp Content-Type = %q, want image/webp", got)
	}

	if err := s.Put(ctx, "k2", bytes.NewReader(pngMagic())); err != nil {
		t.Fatalf("Put png: %v", err)
	}
	if got := m.contentTypes["k2"]; got != "image/png" {
		t.Fatalf("png Content-Type = %q, want image/png", got)
	}
}

func TestS3PutGetDeleteRoundtrip(t *testing.T) {
	m := newMockS3Client()
	s := &S3{client: m, bucket: "b", baseURL: "https://cdn"}
	ctx := context.Background()
	data := []byte("some-image-bytes")

	if err := s.Put(ctx, "key1", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("Get 内容不符: %q", got)
	}
	if err := s.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "key1"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除后 Get 应返回 os.ErrNotExist, got %v", err)
	}
}

func TestS3GetMissingReturnsNotExist(t *testing.T) {
	s := &S3{client: newMockS3Client(), bucket: "b", baseURL: "https://cdn"}
	if _, err := s.Get(context.Background(), "nope"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("期望 os.ErrNotExist, got %v", err)
	}
}

func TestS3Head(t *testing.T) {
	m := newMockS3Client()
	s := &S3{client: m, bucket: "b", baseURL: "https://cdn"}
	ctx := context.Background()
	data := []byte("12345")
	if err := s.Put(ctx, "k", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	size, err := s.Head(ctx, "k")
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("Head size = %d, want %d", size, len(data))
	}
	if _, err := s.Head(ctx, "nope"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Head 不存在应 os.ErrNotExist, got %v", err)
	}
}

// fakeS3Handler 模拟最小 S3 REST API（path-style），用于集成测试。
func fakeS3Handler(t *testing.T, bucket string) http.Handler {
	t.Helper()
	store := map[string][]byte{}
	cts := map[string]string{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			cts[key] = r.Header.Get("Content-Type")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", cts[key])
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(store, key)
			delete(cts, key)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodHead:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", cts[key])
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
}

// TestS3EndToEndPathStyle 用真实 aws-sdk-go-v2 客户端 + 假 S3 服务器，
// 端到端验证 path-style、endpoint 与 Content-Type（R2/MinIO 兼容的关键）。
func TestS3EndToEndPathStyle(t *testing.T) {
	const bucket = "my-bucket"
	srv := httptest.NewServer(fakeS3Handler(t, bucket))
	defer srv.Close()

	s, err := NewS3(S3Options{
		Endpoint:        srv.URL,
		Region:          "auto",
		Bucket:          bucket,
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		PublicBaseURL:   "https://cdn.example.com",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	ctx := context.Background()
	data := webpMagic()
	if err := s.Put(ctx, "key1", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rc, err := s.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("往返内容不符")
	}

	exists, err := s.Exists(ctx, "key1")
	if err != nil || !exists {
		t.Fatalf("Exists = %v, %v", exists, err)
	}
	missing, err := s.Exists(ctx, "nope")
	if err != nil || missing {
		t.Fatalf("Exists(nope) = %v, %v", missing, err)
	}

	if err := s.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "key1"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除后应 os.ErrNotExist, got %v", err)
	}

	if got := s.URL("key1"); got != "https://cdn.example.com/key1" {
		t.Fatalf("URL = %q", got)
	}
}
