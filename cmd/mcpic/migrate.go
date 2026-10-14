package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/storage"
)

// migrateSrc 是迁移源（本地）所需的最小接口。
type migrateSrc interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}

// migrateDst 是迁移目标（S3）所需的最小接口。
type migrateDst interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Head(ctx context.Context, key string) (int64, error)
}

// MigrateFailure 记录单个迁移失败。
type MigrateFailure struct {
	Key string
	Err error
}

// MigrateResult 汇总迁移结果。
type MigrateResult struct {
	Success int
	Skipped int
	Failed  []MigrateFailure
}

// migrateAll 逐条迁移本地图片到对象存储。
// 保证：只读本地、只写云端，从不删除/修改本地原图；单张失败不中断整体；
// 幂等（已存在且非 force 跳过）；可重试（重复运行只补未完成项）。
func migrateAll(ctx context.Context, src migrateSrc, dst migrateDst, keys []string, force, verify bool) MigrateResult {
	var res MigrateResult
	for _, key := range keys {
		_, headErr := dst.Head(ctx, key)
		if headErr == nil && !force {
			res.Skipped++
			continue
		}

		rc, err := src.Get(ctx, key)
		if err != nil {
			res.Failed = append(res.Failed, MigrateFailure{Key: key, Err: fmt.Errorf("读取源文件失败: %w", err)})
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			res.Failed = append(res.Failed, MigrateFailure{Key: key, Err: fmt.Errorf("读取源内容失败: %w", err)})
			continue
		}

		if err := dst.Put(ctx, key, bytes.NewReader(data)); err != nil {
			res.Failed = append(res.Failed, MigrateFailure{Key: key, Err: fmt.Errorf("上传失败: %w", err)})
			continue
		}

		if verify {
			remoteSize, err := dst.Head(ctx, key)
			if err != nil || remoteSize != int64(len(data)) {
				res.Failed = append(res.Failed, MigrateFailure{Key: key, Err: fmt.Errorf(
					"校验失败：对象大小 %d != 本地 %d（err=%v）", remoteSize, len(data), err)})
				continue
			}
		}

		res.Success++
	}
	return res
}

// runMigrate 实现 `mcpic migrate` 子命令：把本地已上传图片安全搬到对象存储。
func runMigrate(force, verify bool) {
	cfg := config.Load()
	if !cfg.IsS3() {
		log.Fatalf("迁移需要设置 MCPIC_STORAGE_TYPE=s3 及全部 MCPIC_S3_* 环境变量（本地原图不删除，可放心运行）")
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置错误：%v", err)
	}

	local, err := storage.NewLocal(cfg.StoragePath)
	if err != nil {
		log.Fatalf("初始化本地源存储失败: %v", err)
	}
	dst, err := newStorage(cfg)
	if err != nil {
		log.Fatalf("初始化 S3 目标存储失败: %v", err)
	}
	s3st, ok := dst.(*storage.S3)
	if !ok {
		log.Fatalf("内部错误：目标存储不是 S3")
	}

	db, err := meta.Open(filepath.Join(cfg.StoragePath, "mcpic.db"))
	if err != nil {
		log.Fatalf("初始化 SQLite 失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	imgs, err := db.List(context.Background(), 500)
	if err != nil {
		log.Fatalf("读取元数据失败: %v", err)
	}
	keys := make([]string, 0, len(imgs))
	for _, img := range imgs {
		keys = append(keys, img.Key)
	}

	log.Printf("开始迁移：共 %d 张（force=%v verify=%v）", len(keys), force, verify)
	res := migrateAll(context.Background(), local, s3st, keys, force, verify)

	log.Printf("迁移完成：成功 %d / 跳过 %d / 失败 %d", res.Success, res.Skipped, len(res.Failed))
	for _, f := range res.Failed {
		log.Printf("  失败 key=%s: %v", f.Key, f.Err)
	}

	// 有失败项时非零退出，便于脚本化重试判定。
	if len(res.Failed) > 0 {
		os.Exit(1)
	}
}
