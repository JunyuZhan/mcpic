# 群晖 NAS + Docker + Cloudflare Tunnel 部署指南

> 适用：MCPic v1.0.0，群晖 DSM 7.2+（Container Manager），Cloudflare Tunnel（cloudflared）。
> 目标：在群晖上长期运行 MCPic HTTP 服务，经 Cloudflare Tunnel 对外提供 HTTPS 链接，
> 并让 AI Agent（Claude Code / Claude Desktop / WorkBuddy 等）通过 MCP 上传 NAS 目录里的图片。

---

## 0. 架构总览与一条铁律

```
AI Agent（电脑）
   │  MCP stdio（方案 A：ssh 到 NAS 执行 mcpic mcp —— 推荐）
   ▼
群晖 NAS
   ├─ mcpic 容器（HTTP :8080 ← 仅回环 127.0.0.1:18080）
   │     └─ volume: /volume1/docker/mcpic/data  →  容器 /data
   └─ cloudflared 容器 ──► Cloudflare Tunnel ──► https://pic.example.com
```

> ### ⚠️ 铁律：MCP 与 HTTP 必须共用同一个 `MCPIC_STORAGE_PATH`，否则链接 404
>
> MCP 进程（`mcpic mcp`）写入图片与 `mcpic.db`，HTTP 服务按 key 读取，二者靠**同一份
> 存储目录**协作。目录不一致时：**MCP 上传会正常返回 URL，但 HTTP 查不到该 key，
> 打开链接 404。**
> - 本指南方案 A（ssh 执行）天然满足：MCP 与 HTTP 同机同目录。
> - 若你改用「电脑本地跑 MCP」（方案 B），必须让两边指向同一份数据（见 §6.2 的风险提示）。

---

## 1. 准备 Docker 镜像

在有本仓库的机器上构建，导出后导入群晖：

```bash
# 1) 构建（仓库根目录）
docker build -t mcpic/mcpic:v1.0.0 .

# 2) 导出为 tar（约 37MB）
docker save mcpic/mcpic:v1.0.0 -o mcpic-v1.0.0.tar

# 3) 导入群晖，二选一
#    a. Container Manager → 映像 → 新增 → 从文件添加（界面导入 mcpic-v1.0.0.tar）
#    b. SSH 到 NAS 后执行：docker load -i mcpic-v1.0.0.tar
```

群晖 SSH：控制面板 → 终端机和 SNMP → 启用 SSH；导入后 `docker images` 应能看到
`mcpic/mcpic:v1.0.0`。

---

## 2. docker-compose.yml（Container Manager 项目）

Container Manager → 项目 → 新增 → 命名 `mcpic` → 从模板创建，粘贴以下内容：

```yaml
services:
  mcpic:
    image: mcpic/mcpic:v1.0.0
    container_name: mcpic
    restart: unless-stopped
    ports:
      # 只绑定本机回环：外网一律走 Cloudflare Tunnel，不暴露 NAS 端口
      - "127.0.0.1:18080:8080"
    environment:
      # ① HTTP 上传接口的鉴权密钥（MCP stdio 不校验此值，它只保护 /api/upload）
      MCPIC_API_KEY: "REPLACE_WITH_openssl_rand_hex_32"
      # ② 存储路径（容器内固定 /data，由下方 volume 映射到 NAS）
      MCPIC_STORAGE_PATH: "/data"
      # ③ 生成链接的对外地址 = 你的 Cloudflare Tunnel 域名（必须 https）
      MCPIC_BASE_URL: "https://pic.example.com"
      # ④ 路径白名单：仅对 `mcpic mcp` 生效（见 §6）；放在这里的值供 ssh 方案复用
      MCPIC_ALLOWED_PATHS: "/volume1/homes/agent/inbox:/volume1/photo/inbox"
    volumes:
      # 数据持久化：图片 + mcpic.db 都在这个目录
      - /volume1/docker/mcpic/data:/data

  cloudflared:
    image: cloudflare/cloudflared:latest
    container_name: cloudflared
    restart: unless-stopped
    command: tunnel --no-autoupdate run
    environment:
      TUNNEL_TOKEN: "REPLACE_WITH_YOUR_TUNNEL_TOKEN"
    depends_on:
      - mcpic
```

> 两个服务在同一项目网络内，cloudflared 可直接用 `http://mcpic:8080` 回源（见 §5）。
> 若你已有在跑的 cloudflared，可删除本文件的 cloudflared 段，改用现有 tunnel 的 ingress 加一条。

---

## 3. 四个环境变量的生产配置建议

| 变量 | 生产建议 | 说明 |
| :--- | :--- | :--- |
| `MCPIC_API_KEY` | `openssl rand -hex 32` 生成 64 位随机串 | 只用于 HTTP `POST /api/upload` 的 `X-API-Key`；**MCP stdio 传输不校验它**（可同时配在 MCP 端但无实际作用）。 |
| `MCPIC_STORAGE_PATH` | 容器内固定 `/data`，宿主机映射 `/volume1/docker/mcpic/data` | **铁律变量**：MCP 端必须指向这同一份数据（方案 A 天然一致）。 |
| `MCPIC_BASE_URL` | `https://pic.example.com`（你的隧道域名，**必须显式配置**） | 不配时：HTTP 上传能靠请求 Host 回推出正确域名，**但 MCP 生成的 URL 会回退成 `http://localhost:8080`**（MCP 没有请求头），Agent 拿到的链接外网打不开。生产环境务必显式设置。 |
| `MCPIC_ALLOWED_PATHS` | 只放真正需要上传的目录，冒号分隔，**不要用 `*`** | 生效位置是 **MCP 进程**，不是 HTTP 容器（HTTP 没有 path 参数）。示例：`/volume1/homes/agent/inbox:/volume1/photo/inbox`。前缀按目录边界匹配，`/volume1/photo` 不会放行 `/volume1/photo-backup`。 |

生成密钥：

```bash
openssl rand -hex 32
```

---

## 4. Cloudflare Tunnel 配置

1. **创建隧道**（Cloudflare Zero Trust → Networks → Tunnels → Create）：
   - 选择 Docker 方式，拿到 `TUNNEL_TOKEN`，填入上方 compose 的 `TUNNEL_TOKEN`。
2. **配置公网域名**（同一页面 → Public Hostname → Add）：
   - Subdomain: `pic`，Domain: `example.com` → 完整域名 `pic.example.com`
   - Service: **HTTP**，URL: **`http://mcpic:8080`**（同 compose 网络内直连，无需宿主机端口）
3. **DNS 自动生效**：Cloudflare 会自动创建 `pic.example.com → 隧道` 的 CNAME，无需公网 IP、无需在路由器开端口。
4. 群晖防火墙无需放行任何端口（mcpic 仅绑定 `127.0.0.1`）。

> 若 cloudflared 跑在 compose 之外（已有 tunnel），在其 `config.yml` 的 `ingress` 中追加：
>
> ```yaml
> - hostname: pic.example.com
>   service: http://172.17.0.1:18080   # 或宿主机局域网 IP:18080
> ```
>
> 并保持 mcpic 的 `MCPIC_BASE_URL=https://pic.example.com`。

---

## 5. 启动与基础验证

```bash
# NAS 上（Container Manager 项目启动后）
curl -s http://127.0.0.1:18080/healthz          # 期望: ok
curl -s https://pic.example.com/healthz         # 期望: ok（经隧道）

# 上传一张图（把 KEY 换成你的 MCPIC_API_KEY）
curl -X POST https://pic.example.com/api/upload \
  -H "X-API-Key: $KEY" -F "file=@/tmp/test.png"
# 期望返回 JSON，url 形如 https://pic.example.com/i/xxxxxxxx
```

浏览器打开返回的 `url`：应看到暗色预览页（含「复制 Markdown」按钮），
追加 `?raw=1` 应返回 `image/webp` 图片字节。

---

## 6. MCP 接入：让 Agent 能上传 NAS 目录

### 6.1 方案 A（推荐）：ssh 远程执行，MCP 进程跑在 NAS 上

这样 `path` 就是 **NAS 路径**，MCP 与 HTTP 同机同目录，铁律天然满足。

**准备：** 把静态二进制放到 NAS（群晖上可直接运行，无需 CGO）：

```bash
# 在电脑上（把 nas 换成你的 NAS 地址）
scp mcpic nas:/tmp/ && ssh nas 'sudo mv /tmp/mcpic /usr/local/bin/mcpic && sudo chmod +x /usr/local/bin/mcpic'
```

**MCP 配置**（Claude Code 项目 `.mcp.json` / Claude Desktop / Cursor 的 mcp.json）：

```json
{
  "mcpServers": {
    "mcpic": {
      "command": "ssh",
      "args": [
        "nas", "-T",
        "env",
        "MCPIC_STORAGE_PATH=/volume1/docker/mcpic/data",
        "MCPIC_BASE_URL=https://pic.example.com",
        "MCPIC_ALLOWED_PATHS=/volume1/homes/agent/inbox:/volume1/photo/inbox",
        "/usr/local/bin/mcpic", "mcp"
      ]
    }
  }
}
```

- `nas` 换成 `~/.ssh/config` 里的主机别名（已配好免密登录）。
- 三个环境变量通过 `env` 在远端注入；`MCPIC_STORAGE_PATH` **必须**与 §2 compose 的
  volume 宿主机目录一致 —— 这正是铁律的落点。
- 之后让 Agent 调用 `upload_image(path="/volume1/homes/agent/inbox/shot.png", max_width=400)` 即可。

**`MCPIC_ALLOWED_PATHS`（方案 A）**：配 NAS 上的目录，例如

```
/volume1/homes/agent/inbox:/volume1/photo/inbox
```

只授予需要上传的目录；调整后重启 MCP 会话生效（每次 `tools/call` 都会重新读环境）。

### 6.2 方案 B：电脑本地跑 MCP（不推荐，注意风险）

MCP 在电脑上执行，`path` 是电脑路径（含 SMB 挂载的 NAS 目录）。
**必须**把 `MCPIC_STORAGE_PATH` 指向与 NAS HTTP 服务同一份数据，例如把 NAS 共享目录挂为
`/Users/me/NAS/mcpic-data`，两边同指该目录 —— 否则 404（铁律）。

> ⚠️ 风险：此时 SQLite（`mcpic.db`）会经 SMB 网络文件系统读写，**网络文件系统上的
> SQLite/WAL 并不可靠**（锁语义、断连损坏）。方案 B 可用于轻量试用，长期部署请用方案 A。

---

## 7. 验收清单

| # | 检查项 | 期望 |
| :--- | :--- | :--- |
| 1 | `curl 127.0.0.1:18080/healthz` | `ok` |
| 2 | `curl https://pic.example.com/healthz` | `ok` |
| 3 | 上传 API（带 Key） | 返回 `https://pic.example.com/i/...` |
| 4 | 浏览器打开该 URL | 预览页 200；`?raw=1` 返回图片 |
| 5 | MCP `upload_image(path=...)` | 返回 URL 且链接可打开（铁律验证） |
| 6 | `upload_image(path=..., max_width=400)` | 下载后宽度 400px（v1.0.0 回归点） |
| 7 | `upload_image(path=..., format="original")` | 下载后格式为原格式 |
| 8 | NAS 重启后 | Container Manager 自动拉起，旧链接仍可访问 |

---

## 8. 故障排查

| 现象 | 原因与处理 |
| :--- | :--- |
| **MCP 返回的链接打开 404** | **铁律被破坏**：MCP 与 HTTP 的 `MCPIC_STORAGE_PATH` 不是同一份数据。对齐两边（方案 A 检查 §6.2 里 `env` 的路径与 compose volume 是否同一 NAS 目录）。 |
| MCP 链接是 `http://localhost:8080/...` | MCP 端没设 `MCPIC_BASE_URL`（MCP 无请求头，无法回退推断）。设为 `https://pic.example.com`。 |
| 外网打不开域名 | cloudflared 日志（Container Manager → cloudflared → 日志）确认 token 与 ingress URL `http://mcpic:8080`；群晖防火墙无需开公网端口。 |
| 上传 401 | `X-API-Key` 与容器 `MCPIC_API_KEY` 不一致。 |
| `路径不在允许范围内` | MCP 端 `MCPIC_ALLOWED_PATHS` 未包含该目录，或目录前缀不匹配（按边界匹配，需给到完整父目录）。 |
| 链接域名不对 / 证书告警 | 确认 `MCPIC_BASE_URL` 是隧道的 https 域名，而不是局域网 IP。 |

---

*v1.0.0 部署指南 · 变更前请先提交 issue 讨论（v1 已冻结）。*
