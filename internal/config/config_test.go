package config

import (
	"strings"
	"testing"
)

func TestLoadDefaultsToLocal(t *testing.T) {
	cfg := Load()
	if cfg.IsS3() {
		t.Fatalf("未设置 MCPIC_STORAGE_TYPE 应视为 local，got %q", cfg.StorageType)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("默认 local 配置不应报错: %v", err)
	}
}

func TestLoadS3Config(t *testing.T) {
	t.Setenv("MCPIC_STORAGE_TYPE", "s3")
	t.Setenv("MCPIC_S3_ENDPOINT", "https://abc.r2.cloudflarestorage.com")
	t.Setenv("MCPIC_S3_REGION", "auto")
	t.Setenv("MCPIC_S3_BUCKET", "my-bucket")
	t.Setenv("MCPIC_S3_ACCESS_KEY_ID", "akid")
	t.Setenv("MCPIC_S3_SECRET_ACCESS_KEY", "secret")
	t.Setenv("MCPIC_S3_PUBLIC_BASE_URL", "https://cdn.example.com")

	cfg := Load()
	if !cfg.IsS3() {
		t.Fatalf("应为 s3 模式")
	}
	if cfg.S3.Endpoint != "https://abc.r2.cloudflarestorage.com" {
		t.Fatalf("endpoint = %q", cfg.S3.Endpoint)
	}
	if cfg.S3.Region != "auto" || cfg.S3.Bucket != "my-bucket" {
		t.Fatalf("region/bucket = %q/%q", cfg.S3.Region, cfg.S3.Bucket)
	}
	if cfg.S3.PublicBaseURL != "https://cdn.example.com" {
		t.Fatalf("public base url = %q", cfg.S3.PublicBaseURL)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("完整 s3 配置不应报错: %v", err)
	}
}

func TestS3ForcePathStyleDefaultsTrue(t *testing.T) {
	t.Setenv("MCPIC_STORAGE_TYPE", "s3")
	cfg := Load()
	if !cfg.S3.ForcePathStyle {
		t.Fatalf("FORCE_PATH_STYLE 默认应为 true（兼容 R2/MinIO）")
	}
}

func TestS3ForcePathStyleExplicitFalse(t *testing.T) {
	t.Setenv("MCPIC_STORAGE_TYPE", "s3")
	t.Setenv("MCPIC_S3_FORCE_PATH_STYLE", "false")
	cfg := Load()
	if cfg.S3.ForcePathStyle {
		t.Fatalf("显式 false 应生效")
	}
}

func TestValidateS3MissingRequired(t *testing.T) {
	cases := []struct {
		name    string
		missing string
	}{
		{"endpoint", "MCPIC_S3_ENDPOINT"},
		{"region", "MCPIC_S3_REGION"},
		{"bucket", "MCPIC_S3_BUCKET"},
		{"access-key", "MCPIC_S3_ACCESS_KEY_ID"},
		{"secret-key", "MCPIC_S3_SECRET_ACCESS_KEY"},
		{"public-base-url", "MCPIC_S3_PUBLIC_BASE_URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MCPIC_STORAGE_TYPE", "s3")
			t.Setenv("MCPIC_S3_ENDPOINT", "https://e")
			t.Setenv("MCPIC_S3_REGION", "auto")
			t.Setenv("MCPIC_S3_BUCKET", "b")
			t.Setenv("MCPIC_S3_ACCESS_KEY_ID", "a")
			t.Setenv("MCPIC_S3_SECRET_ACCESS_KEY", "s")
			t.Setenv("MCPIC_S3_PUBLIC_BASE_URL", "https://cdn")
			t.Setenv(tc.missing, "") // 清掉单个必填项

			err := Load().Validate()
			if err == nil {
				t.Fatalf("缺失 %s 应报错", tc.missing)
			}
		})
	}
}

func TestValidateInvalidStorageType(t *testing.T) {
	t.Setenv("MCPIC_STORAGE_TYPE", "ftp")
	err := Load().Validate()
	if err == nil || !strings.Contains(err.Error(), "ftp") {
		t.Fatalf("非法类型应报错并含类型名, got %v", err)
	}
}

func TestValidateLocalIgnoresMissingS3(t *testing.T) {
	// 即便显式 local，S3 字段全空也不报错。
	t.Setenv("MCPIC_STORAGE_TYPE", "local")
	if err := Load().Validate(); err != nil {
		t.Fatalf("local 模式不应校验 S3 字段: %v", err)
	}
}
