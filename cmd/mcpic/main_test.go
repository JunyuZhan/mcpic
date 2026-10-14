package main

import (
	"testing"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/storage"
)

func TestNewStorageLocal(t *testing.T) {
	cfg := &config.Config{StoragePath: t.TempDir()}
	st, err := newStorage(cfg)
	if err != nil {
		t.Fatalf("newStorage(local): %v", err)
	}
	if _, ok := st.(*storage.Local); !ok {
		t.Fatalf("默认应为 *storage.Local, got %T", st)
	}
}

func TestNewStorageS3(t *testing.T) {
	cfg := &config.Config{
		StoragePath: t.TempDir(),
		StorageType: config.StorageS3,
		S3: config.S3Config{
			Endpoint:        "https://e",
			Region:          "auto",
			Bucket:          "b",
			AccessKeyID:     "a",
			SecretAccessKey: "s",
			PublicBaseURL:   "https://cdn",
			ForcePathStyle:  true,
		},
	}
	st, err := newStorage(cfg)
	if err != nil {
		t.Fatalf("newStorage(s3): %v", err)
	}
	if _, ok := st.(*storage.S3); !ok {
		t.Fatalf("s3 应为 *storage.S3, got %T", st)
	}
}
