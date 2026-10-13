# MCPic API 与 MCP 协作说明

> 适用版本：v1.0.3 · 与线上验收报告（2026-10-13）实测结果同步

---

## 0. 一条铁律（读三遍）

> ### ⚠️ MCP 与 HTTP 必须**同机同存储**（共用同一个 `MCPIC_STORAGE_PATH`），否则返回的链接必然 404

`mcpic mcp` 的工作方式是 **本地写文件 + 写本地 SQLite，然后用 `MCPIC_BASE_URL` 拼一个字符串当作链接返回**。
它**不会把图片 POST 到线上站点**——`MCPIC_BASE_URL` 只影响返回链接的前缀，不影响文件去向。

因此「MCP 返回的链接能否打开」只取决于一件事：

> MCP 进程写的那个存储目录，是否和 HTTP 服务读取的存储目录是**同一个**（含其中的 `mcpic.db`）。

**对照实验实测（2026-10-13）：**

| 场景 | 配置 | 结果 |
| :--- | :--- | :--- |
| S1 ❌ | `BASE_URL=线上域名` + 存储在本机 | MCP 报「上传成功」，链接打开 **404**（`IMAGE_NOT_FOUND`） |
| S2 ✅ | `BASE_URL=localhost` + 存储与本机 HTTP 服务相同 | 链接打开 **200** `image/webp` |
| S3 ✅ | 线上 HTTP 直传的图片 | 链接打开 **200** `image/webp` |

**正确接法（二选一）：**

- **方案 A（推荐）**：MCP 跑在 NAS/服务器上，`MCPIC_STORAGE_PATH` 指向 HTTP 容器挂载的同一宿主目录
  （如 `/volume1/docker/mcpic/data`），用 `ssh` 远程执行 `mcpic mcp`（见 `docs/deploy-synology.md` §6.1）。
- **方案 B（不推荐）**：本机跑 MCP，`MCPIC_STORAGE_PATH` 指向 NAS 的 SMB 挂载目录。
  SQLite 走网络文件系统锁语义不可靠，**有损坏生产库的风险**，仅限轻量试用。

---

## 1. HTTP 接口一览

| 方法 | 路径 | 鉴权 | 说明 |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/upload` | `X-API-Key` | multipart（字段 `file`）或 JSON base64；自动转 WebP(85)、超宽等比缩放 |
| `GET` | `/api/gallery` | `X-API-Key` | D4.7 新增：图片列表 JSON（最近 100 条），字段见下 |
| `DELETE` | `/api/images/{key}` | `X-API-Key` | 删除图片：先删元数据再删文件；不存在返回 `404 IMAGE_NOT_FOUND` |
| `GET` | `/i/{key}` | 无 | 内容协商：`Accept` 含 `text/html` → 预览页；否则图片字节；`?raw=1` 强制原图 |
| `GET` | `/gallery` | 无（页面壳） | D4.7 起为**无数据壳页**，列表经前端持 Key 调 `/api/gallery` 渲染；无 Key 跳 `/upload` |
| `GET` | `/healthz` | 无 | 健康检查，返回 `ok` |
| `GET` | `/api/images/{key}` | — | **不支持 GET**（仅 DELETE），返回 `405`，属设计行为 |

**`GET /api/gallery` 返回示例（线上实测）：**

```json
{"items":[{"key":"2qM5ZNHd","format":"webp","width":800,"height":600,"size":5600,
  "created":"2026-10-13 20:23","url":"https://pic.example.com/i/2qM5ZNHd",
  "markdown":"![image](https://pic.example.com/i/2qM5ZNHd)",
  "html":"<img src=\"https://pic.example.com/i/2qM5ZNHd\" alt=\"image\">",
  "bbcode":"[img]https://pic.example.com/i/2qM5ZNHd[/img]"}]}
```

**错误约定：** 鉴权失败 `401 UNAUTHORIZED`；资源不存在 `404 IMAGE_NOT_FOUND`；参数错误 `400 BAD_REQUEST`
（错误体均含 `error` 错误码与三要素 `message`）。

---

## 2. 上传大小限制与「3MB 预压缩」的生效范围

- **服务端上限固定 10MB**（`MaxUploadBytes`），HTTP API 与网页端一致，超出直接拒绝。
- **「超过 3MB 自动前端预压缩」（长边 1920、WebP 质量 85）仅在网页上传页生效**：
  这是为绕开 Cloudflare 隧道等 ~3MB 级带宽墙的浏览器端体验优化，压缩后体积通常 <1MB。
- **直接调用 `POST /api/upload` 不受 3MB 预压缩影响**（也不做任何隐式压缩，只做服务端标准
  WebP 转换与缩放）：超过隧道带宽的直传会由隧道层报错，属部署环境限制而非 API 限制。
- **若需要在 API 端也做 3MB 级限制或压缩，属于后端改造，标记为 v2 范围**（v1 冻结不动）。

---

## 3. 鉴权说明

- `X-API-Key` 仅保护**写操作与列表**（`POST /api/upload`、`GET /api/gallery`、`DELETE /api/images/{key}`）。
- `MCPIC_API_KEY` 环境变量对 **MCP stdio 传输无校验作用**（MCP 是本地进程，不经 HTTP）——
  配了无害，但不要误以为它保护了 MCP 通道。
- 图片直链 `/i/{key}` 公开可访问（分享场景），预览页与 `?raw=1` 均无需 Key。

---

## 4. 线上响应头基准（实测）

| 头 | 值 |
| :--- | :--- |
| `content-type` | `image/webp` |
| `cache-control` | `public, max-age=31536000, immutable` |
| `x-content-type-options` | `nosniff` |

---

*v1.0.3 · 变更前请先提交 issue 讨论（v1 已冻结）。*
