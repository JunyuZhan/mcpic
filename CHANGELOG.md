# Changelog

本项目所有重要变更都将记录在此文件。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added
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

## [0.1.0] - 未发布

MCPic v1 开发中，目标：通过 PRD 全部 10 条验收标准（A1-A10）。

[Unreleased]: https://github.com/mcpic/mcpic/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mcpic/mcpic/releases/tag/v0.1.0
