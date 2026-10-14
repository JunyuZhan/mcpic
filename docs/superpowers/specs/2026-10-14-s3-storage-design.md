# Spec：S3 兼容对象存储后端（直连模式）

- **日期**：2026-10-14
- **状态**：已批准（人类确认）
- **范围**：v2 首个功能 —— 在 `Storage` 接口上新增第二个实现（S3 兼容协议），图片从本地 NAS 迁往对象存储并改为直连访问，解决本地单点、容量与 CF 隧道带宽墙问题。

## 1. 背景与目标

v1 已 Feature Freeze 并线上验收通过。v2 首个目标为「对象存储后端」，采用通用 S3 兼容协议（endpoint 可配），可接 Cloudflare R2 / 腾讯云 COS / 阿里云 OSS / AWS S3 / MinIO 任意一家。

**核心诉求（人类硬性约束，优先级最高）：**

1. **默认仍是 local**：不设置任何新环境变量时，行为与 v1 完全一致，绝不破坏现有群晖 NAS 部署。
2. **migrate 工具稳定、可重试、绝不丢原图**：把 NAS 已有图片安全搬到云端；失败可重试；任何情况下不删除/不修改本地原图。
3. **兼容 Cloudflare R2 规则**：S3 客户端配置默认走 path-style、签名 v4，确保 R2（后续大概率采用）可直接工作。

## 2. 存储抽象

`Storage` 接口（`internal/storage/storage.go`）四方法签名**不变**（宪法 3.2）：

```go
type Storage interface {
    Put(ctx context.Context, key string, r io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    URL(key string) string
}
```

新增 `S3` 结构体实现，与 `Local` 并列：

| 方法 | S3 实现 | 说明 |
| :--- | :--- | :--- |
| `Put` | `PutObject` | 写入正确 `Content-Type`（直链可直显） |
| `Get` | `GetObject` | 拉流；直连模式 HTTP 层不调用，供 migrate / 回退 |
| `Delete` | `DeleteObject` | 幂等（对象不存在不报错） |
| `URL` | 返回**绝对直链** `PUBLIC_BASE_URL/key` | 见第 3 节 |

`Local` 行为零改动，`URL` 仍返回相对路径 `/i/key`。

### Content-Type 落定

`S3.Put` 内部 `io.ReadAll` → `http.DetectContentType` 检测（webp/png/jpeg/gif 均可靠）→ `PutObject` 携带。图片 ≤10MB 且 service 层已读入内存，无额外成本。不改 Storage 接口签名。

## 3. URL 语义与拼接

**契约变更（已批准）**：`Storage.URL` 语义从「相对路径」放宽为「相对路径或绝对 URL」。

HTTP 层新增辅助函数，统一处理两种形态：

```go
func joinURL(base, u string) string {
    if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
        return u // 已是绝对直链（S3 直连模式）
    }
    return base + u // 相对路径（本地代理模式）
}
```

改动三处拼接点：`upload.go`、`pages.go`、`web.go`（均为 `base + s.svc.URL(key)` → `joinURL(base, s.svc.URL(key))`）。

## 4. 配置

新增环境变量（`internal/config/config.go`），默认全空、行为回退到 local：

| 环境变量 | 说明 | 必填（s3 时） |
| :--- | :--- | :--- |
| `MCPIC_STORAGE_TYPE` | `local`（默认）\| `s3` | — |
| `MCPIC_S3_ENDPOINT` | S3 兼容端点，如 `https://<account_id>.r2.cloudflarestorage.com` | ✅ |
| `MCPIC_S3_REGION` | 区域，R2 用 `auto` | ✅ |
| `MCPIC_S3_BUCKET` | bucket 名 | ✅ |
| `MCPIC_S3_ACCESS_KEY_ID` | Access Key | ✅ |
| `MCPIC_S3_SECRET_ACCESS_KEY` | Secret Key | ✅ |
| `MCPIC_S3_PUBLIC_BASE_URL` | 直链前缀（CDN / r2.dev 域名 / bucket.endpoint） | ✅ |
| `MCPIC_S3_FORCE_PATH_STYLE` | path-style 访问，默认 `true`（兼容 R2/MinIO/COS/OSS） | — |

**校验规则**：`MCPIC_STORAGE_TYPE=s3` 时，任一必填项缺失 → 启动 `log.Fatalf`（fail fast，宪法 7.5 不吞错误）。`MCPIC_STORAGE_TYPE` 非法值同样 fail fast。

**默认 local 保证**：`MCPIC_STORAGE_TYPE` 为空时即 `local`，`Config.Load()` 不读取任何 S3 相关逻辑，行为与 v1 逐字节一致。

## 5. 依赖（宪法 4.5，已批准）

`github.com/aws/aws-sdk-go-v2`（仅 `config`、`credentials`、`service/s3` 三个模块），不引 `feature/s3/manager`（避免过度抽象，宪法 8.3）。

## 6. Cloudflare R2 兼容性（硬约束）

- **path-style**：`aws.Config.UsePathStyle = true`（默认），R2 / MinIO / COS / OSS 均依赖 path-style；AWS S3 原生亦支持（兼容）。
- **签名 v4**：aws-sdk-go-v2 默认 SigV4，R2 支持。
- **region `auto`**：R2 的 region 固定传 `auto`，由 `MCPIC_S3_REGION` 配置。
- **不带 ACL**：`PutObject` 不设 ACL 参数（R2 对部分 ACL 兼容有限），依赖 bucket 级别的公开读设置。
- **直链域名**：R2 公开访问用自定义域名或 `pub-<hash>.r2.dev`，经 `MCPIC_S3_PUBLIC_BASE_URL` 注入，服务不猜测。

## 7. `GET /i/:key` 行为（直连模式）

| 请求 | 本地模式（不变） | 直连模式 |
| :--- | :--- | :--- |
| 浏览器 `Accept: text/html` | 预览页（img 指向 `/i/key?raw=1`） | 预览页（img 指向直链） |
| 图片客户端 / curl | 返回字节流 | **302 → 直链** |
| `?raw=1` | 返回原图字节 | **302 → 直链** |

预览页复制按钮（URL/Markdown）与下载链接均指向直链。`handleImage` 在直连模式下不再调用 `storage.Get`，仅查元数据（SQLite）生成预览页或 302。

## 8. 迁移工具 `mcpic migrate`（硬约束：不丢原图）

子命令：`mcpic migrate`（依赖 `MCPIC_STORAGE_TYPE=s3` 配置 + 本地存储路径）。

**流程**：读 SQLite 全部元数据 → 逐条从本地 `data/` 读文件 → 复用 `storage.S3.Put`（key 不变，Content-Type 由 Put 内部 `http.DetectContentType` 自动检测，与上传链路同一处逻辑）→ 输出进度与结果汇总。

**安全保证（逐条落实「不丢原图」）**：

1. **迁移只读本地、只写云端**：从不删除、从不修改本地原文件。
2. **幂等**：迁移前 `HeadObject` 检查目标是否已存在，已存在则跳过（计数 `skipped`）；`--force` 显式覆盖。
3. **可重试**：单张失败不中断整体，记录失败 key 与原因，继续下一张；重复运行自动跳过已成功项，只补未完成项。
4. **单张原子性**：每张独立，任一失败不影响其他图片与本地原图。
5. **结果报告**：结束打印 `成功 N / 失败 M / 跳过 K`；失败清单逐条列 key 与错误原因。
6. **非零退出码**：存在失败项时进程以非零码退出，便于脚本化重试判定。
7. **可选校验 `--verify`**：迁移后 `HeadObject` 比对 `ContentLength` 与本地字节数，不一致记为失败（不回滚，保留本地原图）。

**迁移后切换**：迁移成功且人工确认后，将 `MCPIC_STORAGE_TYPE=s3` 并重启服务（第 9 节部署）。迁移工具本身不改配置、不删本地。

## 9. 部署

- `docker-compose.yml`：追加 S3 环境变量为注释块（默认不启用，保持 local）。
- `README.md` / `QUICKSTART.md`：补 S3 配置、R2 接入示例、migrate 用法与安全说明。

## 10. 测试计划

| 层 | 内容 |
| :--- | :--- |
| 单元 | config 解析与 s3 校验（缺失项 fail fast）、`S3.URL` 拼接、`joinURL`（绝对/相对）、migrate 逐条逻辑（窄接口 mock S3 客户端） |
| 集成 | httptest 模拟 S3 端点，PUT → HEAD → GET → DELETE 往返；Content-Type 断言；幂等跳过与 `--force` 覆盖 |
| 回归 | 本地模式全链路（上传/访问/画廊/删除）不受影响；`go test ./...` + `make lint` 全绿 |

S3 客户端抽窄接口（`putObject` / `headObject` / `getObject` / `deleteObject`）便于 mock，不引入重量级抽象（宪法 8.3）。

## 11. 明确不做（本 Spec 边界外）

- 对象存储与本地「共存双后端」路由（已否决，走一次性迁移）。
- 预签名 URL、私有 bucket 代理回退（混合模式，已否决）。
- CDN 配置、bucket 创建与公开读策略（属用户在云厂商面板操作）。
- 自动同步/双写。

## 12. 契约变更清单（均已获人类批准）

1. `Storage.URL` 语义放宽：相对路径或绝对 URL。
2. 新增第三方依赖 `aws-sdk-go-v2`（宪法 4.5）。
3. 新增环境变量 `MCPIC_STORAGE_TYPE` 与 `MCPIC_S3_*` 一组（PRD 第 10 节「v1 只需四个环境变量」在 v2 扩展）。
4. 新增子命令 `mcpic migrate`。
