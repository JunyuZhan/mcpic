// Package mcpserver 实现 PRD 3.1 的 MCP 工具 upload_image。
// MCP Server 必须能独立于 HTTP Server 启动（宪法 8.1）：mcpic mcp。
package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mcpic/mcpic/internal/config"
	"github.com/mcpic/mcpic/internal/service"
)

// Server 提供 upload_image 工具。
type Server struct {
	cfg *config.Config
	svc *service.Service
}

// New 构造 MCP Server。
func New(cfg *config.Config, svc *service.Service) *Server {
	return &Server{cfg: cfg, svc: svc}
}

// uploadArgs 是 upload_image 的入参（PRD 3.1：path / url / data 三选一）。
type uploadArgs struct {
	Path     string `json:"path,omitempty" jsonschema:"本地文件的绝对路径（与 url、data 三选一）"`
	URL      string `json:"url,omitempty" jsonschema:"远程图片 URL（与 path、data 三选一）"`
	Data     string `json:"data,omitempty" jsonschema:"base64 编码的图片数据（与 path、url 三选一）"`
	Format   string `json:"format,omitempty" jsonschema:"输出格式：webp（默认）或 original"`
	MaxWidth int    `json:"max_width,omitempty" jsonschema:"最大宽度，超过则等比缩放，默认 1920"`
}

// Run 以 stdio 传输启动 MCP 服务（协议走 stdout，日志必须留在 stderr）。
func (s *Server) Run(ctx context.Context) error {
	return s.Serve(ctx, &mcp.StdioTransport{})
}

// Serve 在给定传输上启动服务，供测试注入 in-memory / command 传输。
func (s *Server) Serve(ctx context.Context, transport mcp.Transport) error {
	server := mcp.NewServer(&mcp.Implementation{Name: "mcpic", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "upload_image",
		Description: "上传图片（本地路径 path、远程 URL url、base64 数据 data 三选一），返回可直接访问的 URL 与 Markdown。",
	}, s.handleUpload)

	return server.Run(ctx, transport)
}

// handleUpload 执行上传。业务失败以 isError 工具结果返回（Agent 可读到完整文案），
// 只有传输层故障才返回 error。
func (s *Server) handleUpload(ctx context.Context, _ *mcp.CallToolRequest, args uploadArgs) (*mcp.CallToolResult, any, error) {
	text := s.upload(ctx, args)
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
	if strings.HasPrefix(text, "上传失败") {
		result.IsError = true
	}
	return result, nil, nil
}

// upload 返回给 Agent 的完整文案（PRD 3.1）。
func (s *Server) upload(ctx context.Context, args uploadArgs) string {
	source, err := pickSource(args)
	if err != nil {
		return renderFailure(err, describeSource(args))
	}

	opts := service.Options{Format: args.Format, MaxWidth: args.MaxWidth}

	var (
		res service.Result
		e   error
	)
	switch source {
	case sourcePath:
		res, e = s.svc.UploadPath(ctx, args.Path)
	case sourceData:
		raw, derr := base64.StdEncoding.DecodeString(args.Data)
		if derr != nil {
			return renderFailure(fmt.Errorf("BAD_REQUEST: data 不是合法的 base64：%v", derr), describeSource(args))
		}
		res, e = s.svc.Upload(ctx, strings.NewReader(string(raw)), "", opts)
	case sourceURL:
		res, e = s.svc.UploadURL(ctx, args.URL, opts)
	}
	if e != nil {
		return renderFailure(e, describeSource(args))
	}

	base := s.baseURL()
	url := base + s.svc.URL(res.Key)
	return fmt.Sprintf("上传成功\nURL: %s\nMarkdown: ![image](%s)", url, url)
}

// baseURL 生成链接基础地址。MCP 请求没有 HTTP 头，只适用 PRD 3.4 的第 1、4 级：
// 显式 MCPIC_BASE_URL，否则 http://localhost:8080。
func (s *Server) baseURL() string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	return "http://localhost:8080"
}
