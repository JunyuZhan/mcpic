// MCPic：面向 AI Agent 的自托管图床。
// PRD v3.2 —— 接收图片、处理存储、返回链接。
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
	"github.com/mcpic/mcpic/internal/meta"
	"github.com/mcpic/mcpic/internal/service"
	"github.com/mcpic/mcpic/internal/storage"
)

func main() {
	cfg := config.Load()

	if err := os.MkdirAll(cfg.StoragePath, 0o750); err != nil {
		log.Fatalf("创建存储目录失败 path=%s: %v", cfg.StoragePath, err)
	}

	st, err := storage.NewLocal(cfg.StoragePath)
	if err != nil {
		log.Fatalf("初始化本地存储失败: %v", err)
	}

	db, err := meta.Open(filepath.Join(cfg.StoragePath, "mcpic.db"))
	if err != nil {
		log.Fatalf("初始化 SQLite 失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if cfg.APIKey == "" {
		log.Printf("警告：未设置 MCPIC_API_KEY，上传接口当前不鉴权（仅建议本地开发使用）")
	}

	svc := service.New(cfg, st, db)
	handler := httpapi.New(cfg, svc).Routes()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("MCPic 已启动，监听 %s（存储路径 %s）", cfg.Addr, cfg.StoragePath)
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
