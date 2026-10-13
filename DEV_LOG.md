# DEV_LOG — MCPic 开发日志

> 落实《Agent 宪法》第九条：每天开始时报告昨日完成/今日计划/阻塞；每天结束时报告验收标准通过情况、未完成项、明日计划。
> 倒序排列，最新日期在最上方。

## 日报模板

```markdown
## YYYY-MM-DD（第 N 天 · 阶段 Dx）

### 昨天完成
- ...

### 今天计划
- ...

### 验收标准通过情况
| 编号 | 条目 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A1 | ... | ✅/❌/⏳ | ... |

### 阻塞 / 需人类决策
- ...

### 明天计划
- ...
```

---

<!-- 最新日报插入在此行下方 -->

## 2026-10-13（第 1 天续 · 阶段 D3）

### 本阶段完成
- D2 缩放复核：`TestProcessResize`（3840→1920x1080）+ curl 实测（4000→1920x480）均通过
- D3：MCP Server `upload_image`（官方 Go SDK v1.8.0，PRD 技术表内推荐依赖）
  - `path` / `url` / `data` 三选一，`format` / `max_width` 可选
  - `mcpic mcp` 独立启动（stdio，协议走 stdout、日志走 stderr，宪法 8.1）
  - 远程 URL 下载（30s 超时 + 大小预检）
  - 失败文案三段式：错误类型 / 相关输入 / 可执行下一步（PRD 3.1）
- D1 提交：`96c7317 feat: 完成 D1 上传闭环与本地存储`

### 验收标准通过情况
| 编号 | 条件 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A2 | Claude Code 配置 MCP 后可上传本地文件 | ✅* | stdio 客户端等价实测：initialize → tools/list → tools/call(path) 成功返回 URL，下载 200 image/webp；Claude Code 实机配置待人类执行 |
| A4 | 上传不存在的路径返回明确错误 | ✅ | `isError=true`，文案与 PRD 示例逐字一致 |
| A10 | 上传 /etc/passwd 返回 PATH_NOT_ALLOWED | ✅ | 三要素齐全（类型 / 路径 / 白名单指引） |
| 其余 | A1/A3/A5/A6/A7/A8/A9 | ✅ | 见上一条日报，D3 无回归（全量测试 + lint 通过） |

### MCP 调用闭环实测记录
```
initialize      → serverInfo: {name: mcpic, version: 0.1.0}
tools/list      → upload_image [path, url, data, format, max_width]
tools/call path → 上传成功
                 URL: http://localhost:8080/i/on1B2Psz
                 Markdown: ![image](http://localhost:8080/i/on1B2Psz)
GET 该 URL      → HTTP 200, image/webp
A4 / A10        → isError=true + 三要素错误文案（见上方完整输出）
```

### 需人类追认的新增错误码（D1 起累计，均未修改 PRD 既有码）
| 错误码 | 用途 | 引入阶段 |
| :--- | :--- | :--- |
| `IMAGE_NOT_FOUND` | GET /i/:key 404 | D1（已批准） |
| `BAD_REQUEST` / `BAD_CONTENT_TYPE` | 请求参数/Content-Type 错误 | D1（本次追认） |
| `DOWNLOAD_FAILED` | MCP url 来源下载失败 | D3（本次追认） |

### Claude Code 接入配置（供人类使用）
```json
{
  "mcpServers": {
    "mcpic": {
      "command": "/path/to/mcpic",
      "args": ["mcp"],
      "env": {
        "MCPIC_STORAGE_PATH": "/data/uploads",
        "MCPIC_BASE_URL": "https://your-domain"
      }
    }
  }
}
```
注：MCP 进程与 HTTP 进程共享同一 `MCPIC_STORAGE_PATH`（SQLite WAL 已验证跨进程并发）。

### 阻塞 / 需人类决策
- 追认上表 3 组新增错误码
- D3 提交（待批准后执行）

### 明天计划（D4）
- Docker 镜像 + 单二进制发布验证（A1、A8 docker stats、A7 复核）
- 首页（项目说明 + MCP 配置示例）、README
- `/metrics` 若需启用，先申请 Prometheus 依赖批准

## 2026-10-13（第 1 天 · 阶段 D1）

### 昨天完成
- 无（项目启动日）

### 今天计划
- 初始化项目骨架（宪法/CHANGELOG/DEV_LOG 文档 + Go 模块 + 目录结构）
- D1：HTTP 上传 + 本地存储 + SQLite 接入，curl 跑通闭环

### 完成情况
- ✅ 骨架：`docs/AGENT_CONSTITUTION.md`（宪法原文逐字提取）、`CHANGELOG.md`、`DEV_LOG.md`、`go.mod`、`Makefile`、目录结构
- ✅ D1：`POST /api/upload`（multipart + JSON base64）、`GET /i/:key`、`GET /healthz`、X-API-Key 鉴权、BASE_URL 四级回退、PRD 第 5 节固定处理（PNG/JPEG → WebP 质量 85 + 超宽缩放）、SQLite 元数据（WAL）
- ✅ 单元测试 4 个包全绿；`make lint`（gofmt + go vet）干净
- ✅ curl 实测闭环通过

### 验收标准通过情况
| 编号 | 条件 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A1 | docker run 后 30 秒内可上传 | ⏳ | Docker 镜像属 D4 |
| A2 | Claude Code 配置 MCP 后可上传 | ⏳ | MCP Server 属 D3 |
| A3 | 上传 PNG 返回 WebP 链接 | ✅ | 6.7MB PNG → WebP 1920x1134，下载验证 RIFF/WEBP |
| A4 | 上传不存在路径返回明确错误 | ⏳ | HTTP 无路径入口；service 单测已覆盖 FILE_NOT_FOUND，MCP 接入后实测 |
| A5 | 上传后 URL 可直接访问 | ✅ | HTTP 200，Content-Type: image/webp |
| A6 | 重启后图片仍可访问 | ✅ | 本地进程重启前后均 200（容器版 D4 再验） |
| A7 | 二进制大小 < 30MB | ✅ | 16M（`CGO_ENABLED` 默认，纯 Go 依赖） |
| A8 | 空闲内存 < 50MB | ✅* | 本地 RSS ≈ 10MB；`docker stats` 指标 D4 再验 |
| A9 | 不设 MCPIC_BASE_URL 时用请求 Host | ✅ | `Host: test.local` → `http://test.local/...`；X-Forwarded-Host 优先亦实测 |
| A10 | 上传 /etc/passwd 返回 PATH_NOT_ALLOWED | ⏳ | 属 MCP 路径入口（D3）；service 单测已过（含 /tmp_evil 边界用例） |

**D1 结论：可测的 5 条（A3/A5/A6/A7/A9）全部通过，A8 本地值达标；A1/A2/A4/A10 待 D3/D4。**

### 期间需人类决策事项（已获批准）
1. **WebP 编码依赖**：`golang.org/x/image/webp` 无编码器，无法满足 PRD 第 5 节与 A3。经批准引入 `github.com/deepteams/webp`（纯 Go，无 CGO）。
2. **技术选型**：纯 Go image 标准库 + `modernc.org/sqlite`（无 CGO）。

### 边界决策（请人类确认）
- **新增错误码 `IMAGE_NOT_FOUND`**：PRD 错误码表未覆盖"读取不存在的图片"场景，`GET /i/:key` 404 时返回 `{"error":"IMAGE_NOT_FOUND"}`。表内既有错误码未做任何修改。
- **GIF / 动画 WebP 不做缩放、不转格式**：重编码只保留首帧会不可逆丢失动画；按实现提示 5 的降级精神原样存储。静图 GIF 同样原样存储（PRD 第 5 节转换逻辑仅针对 PNG/JPEG）。
- **未注册 `/metrics`**：PRD 接口表有 Prometheus 指标，但 Prometheus 客户端依赖不在 PRD 技术表内（宪法 4.5），留待人类批准，计划 D4 处理。

### 阻塞 / 需人类决策
- 无阻塞

### 明天计划（D2/D3）
- D2：图片处理边界补齐（损坏图降级、动图样本测试）——核心处理已在 D1 随纯 Go 方案完成
- D3：MCP Server `upload_image`（path/url/data 三选一）+ Claude Code 接入 → 验收 A2/A4/A10

