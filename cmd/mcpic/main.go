// MCPic：面向 AI Agent 的自托管图床。
// PRD v3.2 —— 接收图片、处理存储、返回链接。
//
// 子命令：
//
//	mcpic        启动 HTTP 服务（默认）
//	mcpic mcp    以 stdio 启动 MCP Server（独立于 HTTP，宪法 8.1）
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/httpapi"
	"github.com/mcpic/mcpic/internal/mcpserver"
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/service"
	"github.com/mcpic/mcpic/internal/storage"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mcp":
			runMCP()
			return
		case "migrate":
			runMigrate(hasFlag("force"), hasFlag("verify"))
			return
		}
	}
	runHTTP()
}

// hasFlag 检查命令行是否包含某个布尔标志（如 --force）。
func hasFlag(name string) bool {
	for _, a := range os.Args[2:] {
		if a == "-"+name || a == "--"+name {
			return true
		}
	}
	return false
}

// setup 读取配置并装配 storage / meta / service。
func setup() (*config.Config, *service.Service, func()) {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置错误：%v", err)
	}

	if err := os.MkdirAll(cfg.StoragePath, 0o750); err != nil {
		log.Fatalf("创建存储目录失败 path=%s: %v", cfg.StoragePath, err)
	}

	st, err := newStorage(cfg)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	db, err := meta.Open(filepath.Join(cfg.StoragePath, "mcpic.db"))
	if err != nil {
		log.Fatalf("初始化 SQLite 失败: %v", err)
	}

	if cfg.APIKey == "" {
		log.Printf("警告：未设置 MCPIC_API_KEY，HTTP 上传接口当前不鉴权（仅建议本地开发使用）")
	}

	svc := service.New(cfg, st, db)
	return cfg, svc, func() { _ = db.Close() }
}

// newStorage 按 StorageType 构造存储后端：s3 或本地（默认）。
func newStorage(cfg *config.Config) (storage.Storage, error) {
	if cfg.IsS3() {
		s := cfg.S3
		return storage.NewS3(storage.S3Options{
			Endpoint:        s.Endpoint,
			Region:          s.Region,
			Bucket:          s.Bucket,
			AccessKeyID:     s.AccessKeyID,
			SecretAccessKey: s.SecretAccessKey,
			PublicBaseURL:   s.PublicBaseURL,
			ForcePathStyle:  s.ForcePathStyle,
		})
	}
	return storage.NewLocal(cfg.StoragePath)
}

// runHTTP 启动 HTTP 服务。
func runHTTP() {
	cfg, svc, cleanup := setup()
	defer cleanup()

	handler := httpapi.New(cfg, svc).Routes()
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("MCPic HTTP 已启动，监听 %s（存储路径 %s）", cfg.Addr, cfg.StoragePath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务启动失败 addr=%s: %v", cfg.Addr, err)
		}
	}()

	<-ctx.Done()
	log.Printf("收到退出信号，正在优雅关闭…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	}
}

// runMCP 以 stdio 启动 MCP Server。协议走 stdout，日志必须留在 stderr。
func runMCP() {
	cfg, svc, cleanup := setup()
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("MCPic MCP Server 已启动（stdio）")
	if err := mcpserver.New(cfg, svc).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("MCP Server 退出: %v", err)
	}
}
