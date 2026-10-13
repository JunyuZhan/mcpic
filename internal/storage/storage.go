// Package storage 定义图片存储抽象。
// PRD 3.3：只有四个方法。新增存储后端只需要实现这四个方法。
package storage

import (
	"context"
	"io"
	"path"
)

// Storage 是 PRD 3.3 定义的不可协商接口。
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

// Local 是本地文件系统实现，文件落在 dir/<key>。
type Local struct {
	dir string
}

// NewLocal 创建本地存储，dir 不存在时自动创建。
func NewLocal(dir string) (*Local, error) {
	if err := mkdirAll(dir); err != nil {
		return nil, err
	}
	return &Local{dir: dir}, nil
}

// Put 写入文件。key 只允许裸文件名（不含路径分隔符），防止路径穿越。
func (l *Local) Put(ctx context.Context, key string, r io.Reader) error {
	if err := checkKey(key); err != nil {
		return err
	}
	return writeAtomic(ctx, l.dir, key, r)
}

// Get 打开文件。文件不存在时返回 os.ErrNotExist。
func (l *Local) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	return openFile(l.dir, key)
}

// Delete 删除文件。文件不存在时返回 os.ErrNotExist。
func (l *Local) Delete(ctx context.Context, key string) error {
	if err := checkKey(key); err != nil {
		return err
	}
	return removeFile(l.dir, key)
}

// URL 返回图片的相对路径；完整链接由 HTTP 层按 BASE_URL 四级回退拼接
// （PRD 3.4），因为 Storage 接口不感知 HTTP 请求。
func (l *Local) URL(key string) string {
	return "/i/" + key
}

// checkKey 拒绝任何带路径分隔符或上级目录的 key。
func checkKey(key string) error {
	if key == "" || key == "." || key == ".." {
		return errInvalidKey
	}
	if path.Base(key) != key {
		return errInvalidKey
	}
	return nil
}
