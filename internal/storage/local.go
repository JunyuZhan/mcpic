package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ErrInvalidKey 表示 key 含路径分隔符或非法字符。
var ErrInvalidKey = errors.New("非法的存储 key")

var errInvalidKey = ErrInvalidKey

func mkdirAll(dir string) error {
	return os.MkdirAll(dir, 0o750)
}

func openFile(dir, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(dir, key))
}

func removeFile(dir, key string) error {
	return os.Remove(filepath.Join(dir, key))
}

// writeAtomic 先写临时文件再 rename，避免读到半截文件。
func writeAtomic(ctx context.Context, dir, key string, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// rename 成功后该文件已不存在，Remove 会返回错误但无影响。
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(dir, key))
}
