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

## 2026-10-13（第 1 天续 · 阶段 D4.8 · 线上上传测试报告修复 · v1.0.3）

### 背景
《MCP 上传测试与外链报告》结论：MCP 配置无法产出可用外链——根因是 MCP 为本地写文件+拼 URL，未同机同存储必然 404（铁律被验证）；另发现 P2 仍在（CF 邮箱混淆把首页 curl 示例的 `file=@screenshot.png` 显示成 `[email protected]`）。用户批准：代码层修 P2、3MB 生效范围标注、文档补铁律与接口实测、打 v1.0.3。P1/P5 由用户在 CF 面板自行处理。

### 修复与产出
| 项 | 内容 |
| :--- | :--- |
| P2 CF 邮箱混淆 | `index.html` curl 示例两处 `@` 改写 HTML 实体 `&#64;screenshot.png`：浏览器渲染与 data-copy 解码均为原样 `@`，CF 字节流扫描不到邮箱形态；`TestHomeDynamicBaseURL` 加防回归断言（须含实体、禁裸 @） |
| 3MB 范围标注 | 首页页脚「超 3MB 网页端预压缩，API 直传不限」；upload 页 footer 同步；`docs/mcp-api.md` 专节声明 API 端限制属 v2 范围 |
| `docs/mcp-api.md`（新建） | 铁律章节（S1 404 / S2 200 / S3 200 对照实验表 + 方案 A/B）、接口一览（含 D4.7 `/api/gallery`、`/gallery` 壳页、`/api/images/{key}` 仅 DELETE 返回 405）、鉴权说明（`MCPIC_API_KEY` 对 MCP stdio 无校验）、线上响应头基准 |
| `QUICKSTART.md` | MCP 链接新增「MCP 不是连到网站上传」提醒；故障排查铁律补对照实验教训（BASE_URL 只改前缀不改去向）；3MB 范围说明；链接 mcp-api.md |

### 验收标准通过情况
| 编号 | 条目 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| D4.8 | P2 字节流无裸 @ | ✅ | curl 实测首页 2 处 `&#64;`、0 处 `file=@screenshot` |
| D4.8 | 渲染与复制不回归 | ✅ | CDP：codeblock 显示 `file=@screenshot.png`；data-copy 解码 `-F 'file=@screenshot.png'` |
| D4.8 | 文档范围与铁律 | ✅ | mcp-api.md/QUICKSTART 落稿；P1/P5 留用户 CF 面板处理 |
| — | `make test`/`lint` | ✅ | 全绿、干净 |

### 阻塞 / 需人类决策
- （无）P1（Always Use HTTPS）、P5（CDN 缓存规则）由用户在 Cloudflare 面板自行调整

### 明天计划
- 打 tag v1.0.3 推送；NAS `git pull && docker compose up -d --build` 后线上复验 P2 显示；CF 面板调整 P1/P5

---

## 2026-10-13（第 1 天续 · 阶段 D4.7 · 线上验收修复）

### 背景（Dogfooding 严格黑盒验收）
用户在 NAS 线上环境黑盒验收发现三类问题，批准 D4.7 修复（Feature Freeze 局部解冻，只修不加）。

### 修复内容
| 编号 | 问题 | 修复 |
| :--- | :--- | :--- |
| P3 | 首页 MCP 配置示例硬编码 `https://mcpic.cn`，用户照抄会拿到死域名 | `index.html` 转 `html/template`，4 处注入 `{{.BaseURL}}`（`MCPIC_BASE_URL` 配置值，空则请求 Host 四级回退）；复制按钮 data-copy JSON 同步动态 |
| P4 | `/gallery` 免鉴权泄露全部图片列表 | 双层防护：① `/gallery` 改无数据壳页（客户端渲染）；② 列表数据仅经 `GET /api/gallery`（`withAuth`）返回；前端 localStorage 无 Key 跳 `/upload`，401 清 Key 回 `/upload`；`/i/{key}` 直链保持公开（可分享场景） |
| 宣传不符 | CF 隧道 ~3MB 带宽墙 vs 文案 10MB | 选预压缩方案保留 10MB：`compressIfLarge()` 对 >3MB 非 GIF 图前端 Canvas 预压缩（长边 1920、`toBlob('image/webp', 0.85)`），解码失败/GIF 原样上传；文案标注「超 3MB 自动预压缩」 |

### 实现要点
- `handleHome` 模板化：`homePageTmpl.Execute(BaseURL: getBaseURL(cfg, r))`
- 新路由 `GET /api/gallery`（withAuth）返回 `{items:[{key,format,width,height,size,created,url,markdown,html,bbcode}]}`
- `gallery.html` 重写为壳页：`load()` 门禁 + fetch 渲染 + 事件委托删除；大小展示前端 `fmtSize`
- `upload.html`：`compressIfLarge` 接入 `pump()` 上传前链路（压缩期间状态显示「上传中」）
- 测试：`TestHomeDynamicBaseURL`（无配置推断 Host / 显式配置用配置值，均断言无 mcpic.cn）、`TestGalleryShellAndAPIAuth`（壳无数据 + API 401×2/200）、`TestGalleryUploadDeleteFlow` 改走 API、`TestUploadPage` 加预压缩断言

### 验收标准通过情况
| 编号 | 条目 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A1–A10 | 既有验收 | ✅ | `make test` 全绿、`make lint` 干净 |
| D4.7 | P3 域名动态注入 | ✅ | curl 实测 `MCPIC_BASE_URL":"https://mcpic.yunzhang.online`，全文 0 处 mcpic.cn |
| D4.7 | P4 画廊鉴权 | ✅ | curl：API 无 key 401 / 有 key 200；CDP：无 key 访问 /gallery 自动跳 /upload，有 key 渲染 44 张 |
| D4.7 | 3MB 预压缩 | ✅ | 前端逻辑 + 文案；线上重建后以真实大图复验 |

### 阻塞 / 需人类决策
- （无）

### 明天计划
- 推送后用户 NAS `git pull && docker compose up -d --build` 重建复验

---

## 2026-10-13（第 1 天续 · 阶段 D4.6 · 多文件队列上传）

### 范围与约束（人类批准，Feature Freeze 局部解冻）
- D4.5 交付后用户实测发现上传页一次只能传一张、无进度、无队列（PRD v3.3 第 9 节仅写「拖拽 + 粘贴」，非遗漏而是最小范围）；经批准开 D4.6
- **后端零改动**：仅改 `static/upload.html` 前端，复用 `/api/upload` 逐张调用
- 串行队列设计动机：避免多请求并发写 SQLite 撞锁

### 实现要点
- `<input multiple>`；拖拽/粘贴/选择三入口全部收集为 `files[]` 入队
- 队列状态机：`pending → uploading → done|fail`，`pump()` 串行取下一张；失败仅标红单张不阻塞后续
- 整体进度：状态行「N/M · 上传中 filename」/「x 成功 · y 失败，可单独重试」
- 结果列表：每张一卡（缩略图 + 元数据 + 四格式复制）；失败卡红色发丝边框 + 重试按钮（重置为 pending 重新入队）
- 非图片 MIME 直接跳过并提示（曾拦下一个 404 JSON 误传入的「图片」，防御生效）
- `TestUploadPage` 断言扩展：multiple / queue / 重试 / data-copy-src（先红后绿）

### 验收标准通过情况
| 编号 | 条目 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A1–A10 | 既有验收 | ✅ | 未动后端 |
| D4.6 | 多选 + 串行队列 + 进度 | ✅ | CDP 实测 3 张连传：状态「3 张上传成功」、3 卡 ×4 格式复制共 12 按钮 |
| D4.6 | 失败单独重试 | ✅ | CDP 实测错 Key 全失败标红 → 修正 Key 点重试 →「1 张上传成功」 |
| D4.6 | 页面要素测试 | ✅ | `TestUploadPage` 扩展断言，全仓 `go test ./...` 绿 |

### 视觉截图
- `/tmp/mcpic-test/d45/multi-upload.png`（3 张结果列表）、`multi-retry.png`（失败标红 + 重试）

### 备忘
- 用户在 NAS 工作副本为 Dockerfile 加了 `ENV GOPROXY=https://goproxy.cn,direct`（国内构建加速，未合并回仓库，冻结不动 Dockerfile）

### 阻塞 / 需人类决策
- （无）

### 明天计划
- 等待用户一周真实使用反馈

---

## 2026-10-13（第 1 天续 · 阶段 D4.5 · 已完成）

### 范围变更（人类指令，PRD v3.2 → v3.3）
- 「前端界面」从 v1 不做移入 v1 做：**基础 Web 上传与画廊管理**（原「管理后台是 v2 的事」废除）
- PRD 升级 v3.3 并重命名 `docs/prd.md` → `docs/PRD.md`；第 2 节（范围）、第 9 节（前端）已按指令重写
- 宪法 1.1 版本引用同步 v3.3（人类已批准，仅改版本号）
- 鉴权决策（人类批准）：页面 GET 公开，写操作（上传/删除 API）必须 `X-API-Key`；前端 localStorage 存 Key，不引入 Cookie/JWT
- 删除语义（人类确认）：**先删 DB 再删文件**；文件删除失败记日志，并在本日报登记「孤儿文件」待清理记录

### 今天计划（D4.5）
1. `meta.Store` 增 `List` / `Delete`，`service` 增 `ListImages` / `DeleteImage` ✅
2. 路由：`GET /upload`（拖拽 + 粘贴 + URL/Markdown/HTML/BBCode 复制）、`GET /gallery`（网格 + 复制 + 删除 + 空状态）、`DELETE /api/images/{key}` ✅
3. 首页/图片页导航互联；DESIGN.md 补两页页面规范 ✅
4. 测试全绿后截图汇报上传闭环 ✅

### 实现要点
- `meta.Insert` 时间戳改为 RFC3339Nano：修复同秒多图画廊排序不稳定（`ORDER BY created_at DESC` 秒级打平）；解析端 `time.Parse(time.RFC3339)` 向后兼容
- `DeleteImage` 严格「先删 DB 再删文件」：文件删除失败仅 `log` 孤儿文件并向上返回成功（DB 已删，不裂图）
- 页面鉴权：`GET /upload`、`GET /gallery` 公开（服务端渲染只读元数据），`DELETE` 走 `withAuth`；前端 Key 存 localStorage
- `//go:embed static/*.html` 通配嵌入，`galleryPageTmpl` 服务端渲染（limit 100）

### 验收标准通过情况
| 编号 | 条目 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A1–A10 | v3.2 既有验收 | ✅ | 见上一条日报，D4.5 未动既有链路 |
| 新增 | 网页上传闭环（D4.5） | ✅ | 测试 `TestUploadPage`/`TestGalleryEmptyState`/`TestGalleryUploadDeleteFlow`（含 401×2、200、404 三要素）/`TestHomeNavToPages`；CDP 实测浏览器上传成功 + 画廊删除 34→33、toast「已删除」 |
| 新增 | 数据层闭环（D4.5） | ✅ | `TestListImagesOrderAndLimit`（倒序/limit/越界收敛）、`TestDeleteImage`、`TestDeleteImageOrphanFileTolerated`（孤儿文件仅记日志） |

### 视觉截图
- Chrome headless + CDP：`/tmp/mcpic-test/d45/{upload,upload-result,gallery,gallery-deleted,home}.png`，三页均符合 DESIGN.md（酸性黄绿唯一主操作、发丝边框、6/12/9999 圆角）

### 孤儿文件登记（删除失败时在此追加）
- （暂无）

### 阻塞 / 需人类决策
- （无）

### 明天计划
- D4.5 截图汇报 → 请求提交 → 等待群晖部署反馈

### 冻结状态（Feature Freeze）
- **v1 Feature Freeze 正式宣布**：`feat: 完成 D4.5 Web 上传页与画廊页`（17 文件）提交后打 tag **v1.0.1**
- 冻结后仅接受：真实使用反馈引发的**缺陷修复**；不新增功能、不开 v2 分支
- 后续路线：群晖 NAS Docker 部署（`docs/deploy-synology.md`）→ 一周真实使用 → 收集反馈后决定是否重启开发

---

## 2026-10-13（第 1 天续 · 阶段 D4）

### 本阶段完成
- Docker 镜像（A1）：`Dockerfile` 多阶段构建（golang:1.26-alpine → alpine + CA 证书，`CGO_ENABLED=0`，非 root 用户，`/data` volume）+ `.dockerignore`
- `DESIGN.md`：基于 Refero "Linear — midnight precision instrument" 暗色系规范裁剪（tokens / 字体 / 间距 / 组件 / 页面 / Do-Don't），适配原生 HTML 单二进制约束
- 原生页面（PRD 第 9 节，无前端构建链）：
  - 首页 `GET /`：项目说明、三步快速开始、**MCP 配置示例**、API 速查、页脚
  - 图片页 `GET /i/:key`：内容协商（`Accept: text/html` → 预览页；否则图片字节，`?raw=1` 强制原图），图片预览 + 元数据 + **一键复制 Markdown**/URL
  - `go:embed` 嵌入二进制；`GET /{$}` 精确匹配避免 catch-all
- `README.md`：Docker/本地启动、环境变量表、MCP 配置、API 示例
- 新增测试：`web_test.go`（首页要素、图片页 HTML、`image/*` 仍返回图片字节）
- 视觉验收：Chrome headless 截图两页，修复代码块复制按钮与首行文字重叠（按钮移至卡片标题行）

### 验收标准全量回归
| 编号 | 条件 | 结果 | 说明 |
| :--- | :--- | :--- | :--- |
| A1 | `docker run` 后 30 秒内可上传 | ✅ | 容器启动 0s 完成 healthz+上传，返回 200 image/webp |
| A2 | Claude Code 配置 MCP 后可上传本地文件 | ✅* | stdio 等价实测通过；Claude Code 实机配置待人类执行 |
| A3 | 上传 PNG 返回 WebP 链接 | ✅ | 7MB PNG → 669,670B WebP，下载 `RIFF` |
| A4 | 不存在的路径返回明确错误 | ✅ | `isError=true`，三要素文案与 PRD 3.1 一致 |
| A5 | URL 可直接打开 | ✅ | 浏览器（Accept: text/html）→预览页；渲染器/直连→图片字节 |
| A6 | 重启容器后图片仍可访问 | ✅ | `docker restart` 后 200 image/webp 与预览页均正常 |
| A7 | 二进制 < 30MB | ✅ | `mcpic` = 23M（`-ldflags "-s -w"`） |
| A8 | 空闲内存 < 50MB | ✅ | `docker stats` 8.0MiB（上传后 15.8MiB） |
| A9 | 不设 BASE_URL 用请求 Host | ✅ | `Host: test.local` → URL 以 `http://test.local` 开头 |
| A10 | `/etc/passwd` 返回 PATH_NOT_ALLOWED | ✅ | 三要素齐全（含 `MCPIC_ALLOWED_PATHS` 指引） |

### 全量质量门禁
- `gofmt -l`（空）+ `go vet` 通过；`go test ./... -count=1` 5 包全绿
- 镜像 `mcpic/mcpic:test` 构建成功（37.1MB）

### 阻塞 / 需人类决策
- **D4 提交批准**：等待人类批准后执行 `git add` + `feat: 完成 D4 Docker 镜像与原生页面`（宪法第十条）
- A2 实机：需要在 Claude Code 中写入 MCP 配置（README 已给出示例）

### 明天计划
- 人类确认 D4 提交；A2 Claude Code 实机验收；如需发布打 tag

---

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

