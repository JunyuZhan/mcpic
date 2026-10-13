package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPutGetDeleteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLocal(dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	ctx := context.Background()
	data := []byte("hello")

	if err := l.Put(ctx, "abc123", bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := l.Get(ctx, "abc123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("Get 内容不符: %q", got)
	}
	if err := l.Delete(ctx, "abc123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := l.Get(ctx, "abc123"); !os.IsNotExist(err) {
		t.Fatalf("删除后应不存在, got %v", err)
	}
}

func TestURL(t *testing.T) {
	l, _ := NewLocal(t.TempDir())
	if got := l.URL("abc"); got != "/i/abc" {
		t.Fatalf("URL = %q", got)
	}
}

func TestKeyTraversalRejected(t *testing.T) {
	l, _ := NewLocal(t.TempDir())
	ctx := context.Background()
	for _, key := range []string{"../evil", "a/b", "", ".", ".."} {
		if err := l.Put(ctx, key, bytes.NewReader([]byte("x"))); err != ErrInvalidKey {
			t.Errorf("key %q 应被拒绝, got %v", key, err)
		}
		if _, err := l.Get(ctx, key); err != ErrInvalidKey {
			t.Errorf("Get key %q 应被拒绝, got %v", key, err)
		}
	}
}

func TestLocalGetMissing(t *testing.T) {
	l, _ := NewLocal(t.TempDir())
	if _, err := l.Get(context.Background(), "nope"); !os.IsNotExist(err) {
		t.Fatalf("期望 os.ErrNotExist, got %v", err)
	}
}

func TestNewLocalCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	if _, err := NewLocal(dir); err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("目录未创建: %v", err)
	}
}
