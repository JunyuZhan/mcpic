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

> ⚠️ **MCP 不是「连到网站上传」**：`mcpic mcp` 在**本地**写文件和数据库，再用 `MCPIC_BASE_URL` 拼链接。
> 链接能打开的前提是 MCP 与 HTTP **同机同存储**（见第 4 节铁律）。接口细节见 [docs/mcp-api.md](docs/mcp-api.md)。

## 3. curl 一句话验证

```bash
curl -X POST http://localhost:8080/api/upload -H "X-API-Key: your-secret-key" -F "file=@your-image.png"
```

返回 JSON 中的 `markdown` 字段即为可直接粘贴的图片链接（如 `![image](http://localhost:8080/i/xxxxxxx)`）。

> 上传上限固定 **10MB**。网页上传页对超过 3MB 的图片会**自动前端预压缩**（长边 1920、WebP 85）以适配
> CF 隧道等窄带宽场景；**直接调 API 不做 3MB 预压缩**（API 端限制属 v2 范围）。

## 4. 故障排查

> ### ⚠️ MCP 与 HTTP 必须共用同一个 `MCPIC_STORAGE_PATH`，否则链接 404
>
> MCP 进程（`mcpic mcp`）和 HTTP 服务是**两个独立进程**，它们靠**同一个存储目录**
> （`MCPIC_STORAGE_PATH`，含其中的 `mcpic.db`）协作：MCP 写入图片与元数据，HTTP 按 key 读取。
> 两边目录不一致时，**MCP 上传会正常返回 URL，但 HTTP 在数据库里查不到该 key，打开链接就是 404。**
>
> **线上对照实验教训**：把 `MCPIC_BASE_URL` 设成线上域名、但存储留在本机/别处 —— MCP 会报
> 「上传成功」并返回一个线上 URL，但该链接必然 404。`BASE_URL` 只改链接前缀，不改文件去向。
>
> 排查方法：对比 MCP 配置里 `MCPIC_STORAGE_PATH` 与 HTTP 服务启动时的 `MCPIC_STORAGE_PATH`
> （Docker 部署时即挂载的同一个 volume，例如 `-v mcpic-data:/data` 对应 `-e MCPIC_STORAGE_PATH=/data`）。
> 不确定时，让两边都指向同一绝对路径即可。
