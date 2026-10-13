# QUICKSTART — MCPic 快速开始

## 1. Docker 一行部署

```bash
docker run -d --name mcpic -p 8080:8080 -v mcpic-data:/data -e MCPIC_API_KEY=your-secret-key -e MCPIC_STORAGE_PATH=/data -e MCPIC_BASE_URL=http://localhost:8080 -e MCPIC_ALLOWED_PATHS=/tmp:/home:/Users mcpic/mcpic:latest
```

> 本地尚无镜像时先 `docker build -t mcpic/mcpic:latest .`
> `MCPIC_BASE_URL` 按实际对外地址修改（不设则自动按请求 Host 推断）；`MCPIC_ALLOWED_PATHS` 限制 MCP `path` 参数可读目录，`*` 表示全部放行。

## 2. MCP 配置（Claude Code / Cursor）

Claude Code（`~/.claude.json`，或项目内 `.mcp.json`）：

```json
{
  "mcpServers": {
    "mcpic": {
      "command": "docker",
      "args": ["exec", "-i", "mcpic", "/usr/local/bin/mcpic", "mcp"],
      "env": {
        "MCPIC_API_KEY": "your-secret-key",
        "MCPIC_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

若已把 `mcpic` 放到 `PATH`（如 `/usr/local/bin/mcpic`），改用本地进程更简单：

```json
{
  "mcpServers": {
    "mcpic": {
      "command": "/usr/local/bin/mcpic",
      "args": ["mcp"],
      "env": {
        "MCPIC_API_KEY": "your-secret-key",
        "MCPIC_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

Cursor 写入 `~/.cursor/mcp.json`，内容相同。配置后让 Agent 调用 `upload_image(path="/tmp/your-image.png")` 即可返回 URL 与 Markdown。

## 3. curl 一句话验证

```bash
curl -X POST http://localhost:8080/api/upload -H "X-API-Key: your-secret-key" -F "file=@your-image.png"
```

返回 JSON 中的 `markdown` 字段即为可直接粘贴的图片链接（如 `![image](http://localhost:8080/i/xxxxxxx)`）。
