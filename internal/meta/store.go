// Package meta 管理图片元数据，存储于 SQLite（零配置，PRD 第 6 节）。
package meta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // 纯 Go 驱动，无 CGO
)

// ErrNotFound 表示 key 不存在。
var ErrNotFound = errors.New("图片记录不存在")

// Image 是一条图片元数据。
type Image struct {
	Key       string
	Format    string
	Size      int64
	Width     int
	Height    int
	CreatedAt time.Time
}

// Store 封装 SQLite 访问。HTTP Handler 不直接触达数据库（宪法 8.1）。
type Store struct {
	db *sql.DB
}

// Open 打开（必要时创建）数据库并建表。
// 实现提示 3：显式开启 WAL 与 busy_timeout，避免 HTTP/MCP 并发写入报 database is locked。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	// modernc 驱动是单写者模型，限制为 1 个连接可避免写锁竞争。
	db.SetMaxOpenConns(1)

	const schema = `
CREATE TABLE IF NOT EXISTS images (
	key        TEXT PRIMARY KEY,
	format     TEXT    NOT NULL,
	size       INTEGER NOT NULL,
	width      INTEGER NOT NULL,
	height     INTEGER NOT NULL,
	created_at TEXT    NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化 SQLite 表结构失败: %w", err)
	}
	return &Store{db: db}, nil
}

// Insert 写入一条元数据。时间用 RFC3339Nano 保证同秒内多条记录仍可稳定倒序。
func (s *Store) Insert(ctx context.Context, img Image) error {
	const q = `INSERT INTO images (key, format, size, width, height, created_at)
	           VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		img.Key, img.Format, img.Size, img.Width, img.Height, img.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("写入元数据失败: %w", err)
	}
	return nil
}

// Get 按 key 查询元数据，不存在时返回 ErrNotFound。
func (s *Store) Get(ctx context.Context, key string) (Image, error) {
	const q = `SELECT key, format, size, width, height, created_at FROM images WHERE key = ?`
	var img Image
	var created string
	err := s.db.QueryRowContext(ctx, q, key).Scan(&img.Key, &img.Format, &img.Size, &img.Width, &img.Height, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	if err != nil {
		return Image{}, fmt.Errorf("查询元数据失败: %w", err)
	}
	if img.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
		return Image{}, fmt.Errorf("解析元数据时间失败: %w", err)
	}
	return img, nil
}

// List 按创建时间倒序返回最近 limit 条元数据（画廊页用）。
func (s *Store) List(ctx context.Context, limit int) ([]Image, error) {
	const q = `SELECT key, format, size, width, height, created_at
	           FROM images ORDER BY created_at DESC, key DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("查询元数据列表失败: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Image
	for rows.Next() {
		var img Image
		var created string
		if err := rows.Scan(&img.Key, &img.Format, &img.Size, &img.Width, &img.Height, &created); err != nil {
			return nil, fmt.Errorf("读取元数据失败: %w", err)
		}
		if img.CreatedAt, err = time.Parse(time.RFC3339, created); err != nil {
			return nil, fmt.Errorf("解析元数据时间失败: %w", err)
		}
		out = append(out, img)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历元数据失败: %w", err)
	}
	return out, nil
}

// Delete 删除一条元数据，不存在时返回 ErrNotFound。
func (s *Store) Delete(ctx context.Context, key string) error {
	const q = `DELETE FROM images WHERE key = ?`
	res, err := s.db.ExecContext(ctx, q, key)
	if err != nil {
		return fmt.Errorf("删除元数据失败: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("确认删除结果失败: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	return s.db.Close()
}
