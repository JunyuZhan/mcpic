# Changelog

本项目所有重要变更都将记录在此文件。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [v1.0.3] - 2026-10-13

### Fixed
- Cloudflare 邮箱混淆误伤首页 curl 示例（P2，D4.8）：`file=@screenshot.png` 的 `@` 改写为 HTML 实体 `&#64;`，浏览器渲染与复制按钮解码不变，CF 扫描不再误判为邮箱地址
- 线上验收问题（D4.7）：
  - P3 首页 MCP 配置示例硬编码 `https://mcpic.cn` → 服务端模板注入 `MCPIC_BASE_URL`（配置值或请求 Host 推断）
  - P4 画廊隐私泄露：`/gallery` 改为无数据壳页，图片列表仅经鉴权 API `GET /api/gallery`（`X-API-Key`）返回；前端 localStorage 无 Key 强制跳转 `/upload`
  - 3MB 隧道带宽墙：上传页对超过 3MB 的非 GIF 图片自动前端预压缩（长边 1920、WebP 质量 85），文案说明预压缩行为

### Added
- `docs/mcp-api.md`：HTTP/MCP 接口文档、线上实测结果、**「MCP 与 HTTP 必须同机同存储」铁律**（含 S1/S2/S3 对照实验）、3MB 预压缩生效范围声明（仅网页端，API 端限制标记 v2）
- `QUICKSTART.md`：补「MCP 不是连到网站上传」语义提醒与 3MB 预压缩范围说明，链接至 mcp-api.md
- 上传页多文件队列（D4.6）：文件选择/拖拽/粘贴支持多选，前端串行队列逐张上传（避免并发写 SQLite），整体进度「N/M」展示，失败项单独标红并支持逐张重试；结果为每张四格式复制（URL/Markdown/HTML/BBCode）。纯前端改动，后端零变化
- 项目文档：PRD v3.2、Agent 宪法（`docs/AGENT_CONSTITUTION.md`）、CHANGELOG、开发日志模板
- HTTP 上传接口 `POST /api/upload`（multipart 与 JSON base64），X-API-Key 鉴权
- 图片访问接口 `GET /i/:key` 与健康检查 `GET /healthz`
- 本地文件存储（Storage 接口，原子写入，key 防穿越）
- SQLite 元数据层（WAL 模式，`modernc.org/sqlite` 纯 Go 驱动）
- 图片处理：魔数识别、尺寸探测、PNG/JPEG → WebP（质量 85）、超宽等比缩放（PRD 第 5 节）
- BASE_URL 四级回退（MCPIC_BASE_URL → X-Forwarded-Host → Host → localhost）
- 本地路径白名单校验（PRD 3.5，Abs+Clean 边界匹配）
- MCP Server `upload_image` 工具（path / url / data 三选一，`mcpic mcp` 独立启动，官方 Go SDK）
- 远程 URL 下载上传（超时与大小预检）
- 三段式失败文案：错误类型 / 相关输入 / 下一步建议（PRD 3.1）
- 单元测试（storage / imgproc / service / httpapi）与 Makefile（build / test / lint / run）
- Docker 多阶段镜像：`Dockerfile`（静态二进制 + CA 证书 + 非 root + `/data` volume）与 `.dockerignore`
- 原生 HTML 首页（项目说明、快速开始、MCP 配置示例、API 速查）与图片预览页（一键复制 Markdown），`go:embed` 嵌入、`Accept` 内容协商、`?raw=1` 取原图
- `DESIGN.md` 设计规范（暗色系，基于 Refero Linear 风格裁剪）与 `README.md` 部署/MCP 配置文档
- 页面与内容协商测试（`web_test.go`）

### Changed
- **v1 范围调整：PRD v3.2 → v3.3**（`docs/prd.md` 重命名为 `docs/PRD.md`）
  - 前端界面从「v1 不做」移入「v1 做」：新增基础 Web 上传页（拖拽 + 粘贴）与画廊页（网格展示、复制链接、删除）
  - 废除「管理后台是 v2 的事」限制；首页 MCP 配置区块即配置页
  - 技术约束不变：原生 `html/template` + 内联 CSS + 原生 JS + `go:embed` 单二进制，禁止 React/Vue/Vite 构建工具链；UI 严格遵循 `DESIGN.md`
  - 宪法 1.1 的 PRD 版本引用同步为 v3.3（人类已批准）

## [0.1.0] - 未发布

MCPic v1 开发中，目标：通过 PRD 全部 10 条验收标准（A1-A10）。

[Unreleased]: https://github.com/mcpic/mcpic/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mcpic/mcpic/releases/tag/v0.1.0
