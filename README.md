# MCPic

面向 AI Agent 的自托管图床：接收图片、自动格式转换与缩放、返回可直接引用的链接与 Markdown。

- 单二进制、零外部依赖（内嵌 SQLite），Docker 一键部署
- 上传自动转 WebP（质量 85），超宽等比缩放（默认 max_width 1920）
- 同时提供 HTTP 上传 API 与 MCP Server（`upload_image` 工具）
- 需求与验收标准见 [docs/prd.md](docs/prd.md)（PRD v3.2），设计规范见 [DESIGN.md](DESIGN.md)

## 快速开始

### Docker（推荐）

```bash
docker run -d -p 8080:8080 \
  -v mcpic-data:/data \
  -e MCPIC_API_KEY=your-secret-key \
  mcpic/mcpic:latest
```

本地构建镜像：

```bash
docker build -t mcpic/mcpic:latest .
```

### 本地运行

```bash
make build
MCPIC_API_KEY=your-secret-key ./mcpic
```

## 配置（环境变量）

| 变量 | 必填 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `MCPIC_API_KEY` | 建议 | 空（放行） | `X-API-Key` 鉴权；留空则本地零配置启动 |
| `MCPIC_STORAGE_PATH` | 否 | `./data` | 图片与 SQLite 存储目录（Docker 镜像内为 `/data`） |
| `MCPIC_BASE_URL` | 否 | 自动 | 生成链接的对外地址；四级回退 `MCPIC_BASE_URL → X-Forwarded-Host → Host → http://localhost:8080` |
| `MCPIC_ALLOWED_PATHS` | 否 | `/tmp:/home:/Users` | 允许上传的本地绝对路径白名单；`*` 放行全部（MCP `path` 参数用） |

## MCP 配置

让 Claude Code 等 Agent 调用 `upload_image`（stdio 传输，独立于 HTTP 服务）：

```json
{
  "mcpServers": {
    "mcpic": {
      "command": "/usr/local/bin/mcpic",
      "args": ["mcp"],
      "env": {
        "MCPIC_STORAGE_PATH": "/data/uploads",
        "MCPIC_API_KEY": "your-secret-key",
        "MCPIC_BASE_URL": "https://mcpic.cn"
      }
    }
  }
}
```

`upload_image` 支持三种来源，返回 `url` 与可直接粘贴的 `markdown`：

- `path`：本机绝对路径（受 `MCPIC_ALLOWED_PATHS` 限制）
- `url`：远程图片 URL
- `data`：base64 编码
- 可选 `format`（`webp` 默认 / `original` 不转换）、`max_width`（默认 1920）

## HTTP API

```bash
# 上传（multipart 或 JSON base64）
curl -X POST http://localhost:8080/api/upload \
  -H "X-API-Key: your-secret-key" \
  -F "file=@screenshot.png"

# 响应
{
  "url": "https://mcpic.cn/i/6PXVGEcf",
  "markdown": "![image](https://mcpic.cn/i/6PXVGEcf)",
  "size": 5600, "width": 800, "height": 600, "format": "webp"
}

# 访问图片：浏览器打开返回预览页，渲染器/直连返回图片字节
curl http://localhost:8080/i/6PXVGEcf
# 强制取原图字节
curl "http://localhost:18080/i/6PXVGEcf?raw=1"

# 健康检查
curl http://localhost:8080/healthz
```

限制：单文件 ≤ 10MB；仅 PNG / JPEG（GIF 与动画 WebP 原样存储，不缩放不转格式）。

## 故障排查

> ### ⚠️ MCP 与 HTTP 必须共用同一个 `MCPIC_STORAGE_PATH`，否则链接 404
>
> MCP 进程（`mcpic mcp`）与 HTTP 服务是两个独立进程，靠**同一个存储目录**（含其中的
> `mcpic.db`）协作：MCP 写入图片与元数据，HTTP 按 key 读取。
> 两边 `MCPIC_STORAGE_PATH` 不一致时，**MCP 上传会正常返回 URL，但 HTTP 查不到该 key，
> 打开链接就是 404。**
>
> 排查：对比 MCP 配置与 HTTP 服务启动时的 `MCPIC_STORAGE_PATH` 是否指向同一绝对路径
> （Docker 部署时即同一个 volume，如 `-v mcpic-data:/data` 配 `-e MCPIC_STORAGE_PATH=/data`）。

## 开发

```bash
make test   # 全量测试
make lint   # gofmt + go vet
make build  # 输出 ./mcpic
```

- 需求：[docs/prd.md](docs/prd.md)（PRD v3.2）
- 行为准则：[docs/AGENT_CONSTITUTION.md](docs/AGENT_CONSTITUTION.md)
- 变更记录：[CHANGELOG.md](CHANGELOG.md)；开发日志：[DEV_LOG.md](DEV_LOG.md)

## License

AGPL-3.0-only
