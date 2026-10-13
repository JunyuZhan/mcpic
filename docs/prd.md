# MCPic 完整文档

**版本**：v3.2
**日期**：2026-10-13
**许可证**：AGPL-3.0-only


# 第一部分：产品需求文档（PRD）

## 1. 产品定义

MCPic 是一个自托管图床，核心能力只有三件事：

1. **接收图片**：从本地路径、URL 或 HTTP 流
2. **处理并存储**：可选的格式转换与缩放，存储到本地或对象存储
3. **返回链接**：URL + Markdown，附带元信息

**核心洞察**：AI Agent 不需要“图床”，它需要的是一个**可靠的、可预测的、错误信息明确的**图片存储接口。

**一句话定位**：让 AI Agent 上传图片像写文件一样自然。


## 2. 范围

### v1 做

- MCP Server，AI Agent 可通过本地路径直传
- HTTP 上传接口
- 本地文件存储
- 上传时格式转换（PNG/JPEG → WebP）与等比缩放
- 简单的图片访问页面
- Docker 镜像与单二进制发布

### v1 不做

- 多用户、注册、登录
- 管理后台
- 防盗链
- WebDAV / OSS / COS / 七牛 / S3
- 水印、滤镜、裁剪、EXIF 编辑
- AI 打标、OCR、NSFW 检测
- 实时 URL 参数处理
- Webhook、邮件通知
- API Token 管理（v1 用环境变量密钥）
- 前端框架、构建工具链

**v1 的目标是：装上就能用，Agent 能上传，链接能访问。**


## 3. 核心接口

### 3.1 MCP 工具：`upload_image`

| 参数 | 类型 | 必填 | 说明 |
| :--- | :--- | :--- | :--- |
| `path` | string | 三选一 | 本地文件绝对路径 |
| `url` | string | 三选一 | 远程图片 URL |
| `data` | string | 三选一 | base64 编码的图片数据 |
| `format` | string | 否 | `webp` / `original`，默认 `webp` |
| `max_width` | int | 否 | 最大宽度，默认 1920 |

**成功返回**：

```
上传成功
URL: https://mcpic.cn/i/abc123
Markdown: ![image](https://mcpic.cn/i/abc123)
```

**失败返回**：

```
上传失败：文件不存在
路径: /tmp/screenshot.png
请检查路径是否正确。
```

失败信息必须包含**三个要素**：错误类型、相关输入、可执行的下一步。

### 3.2 HTTP API

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| POST | `/api/upload` | 上传，接受 multipart 或 JSON |
| GET | `/i/:key` | 访问图片 |
| GET | `/healthz` | 健康检查 |
| GET | `/metrics` | Prometheus 指标 |

**没有 RESTful 风格、分页、过滤、批量操作。** Agent 不关心这些。

**上传请求示例**：

```bash
curl -X POST https://mcpic.cn/api/upload \
  -H "X-API-Key: $MCPIC_API_KEY" \
  -F "file=@screenshot.png" \
  -F "format=webp" \
  -F "max_width=1920"
```

**响应**：

```json
{
  "url": "https://mcpic.cn/i/abc123",
  "markdown": "![image](https://mcpic.cn/i/abc123)",
  "size": 184320,
  "width": 1920,
  "height": 1080,
  "format": "webp"
}
```

### 3.3 存储接口

```go
type Storage interface {
    Put(ctx context.Context, key string, r io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    URL(key string) string
}
```

**只有四个方法。** 没有 PresignURL、没有 HealthCheck、没有 Exists——v1 不需要。新增存储后端只需要实现这四个方法。

### 3.4 BASE_URL 动态获取

生成链接时按以下优先级回退：

1. 如果设置了 `MCPIC_BASE_URL`，始终使用它（显式配置优先）
2. 否则，读取请求的 `X-Forwarded-Host` 头（反向代理场景）
3. 否则，读取请求的 `Host` 头
4. 都没有，回退到 `http://localhost:8080`

**约束**：v1 不引入“自动检测域名”的复杂逻辑，只做这个四级回退。

### 3.5 本地路径安全边界

MCP Server 允许传绝对路径，必须限制路径范围。新增环境变量 `MCPIC_ALLOWED_PATHS`，默认值为 `/tmp:/home:/Users`。

- 如果路径不在白名单内，返回 `PATH_NOT_ALLOWED` 错误
- 错误信息格式：`路径 /etc/passwd 不在允许范围内，允许的路径前缀：/tmp, /home, /Users`
- 如果用户显式设置 `MCPIC_ALLOWED_PATHS=*`，则允许所有路径（自担风险）

**约束**：v1 不做文件类型白名单，只做路径范围限制。


## 4. 错误处理

Agent 需要知道**发生了什么**和**该做什么**。每个错误必须指向具体原因。

| 错误码 | 触发条件 | 返回信息 |
| :--- | :--- | :--- |
| `FILE_NOT_FOUND` | 路径不存在 | `路径 /tmp/a.png 不存在，请检查路径` |
| `UNSUPPORTED_FORMAT` | 格式不支持 | `不支持 .bmp 格式，支持：jpg/png/webp/gif` |
| `FILE_TOO_LARGE` | 超过大小限制 | `文件 15MB 超过限制 10MB` |
| `STORAGE_ERROR` | 存储写入失败 | `本地存储写入失败：磁盘空间不足` |
| `PROCESS_ERROR` | 图片处理失败 | `图片处理失败：图片可能已损坏` |
| `UNAUTHORIZED` | API Key 无效 | `API Key 无效或缺失` |
| `PATH_NOT_ALLOWED` | 路径不在白名单内 | `路径 /etc/passwd 不在允许范围内，允许的路径前缀：/tmp, /home, /Users` |

**没有模糊的 “internal error”。**


## 5. 图片处理

v1 的处理逻辑固定为：

```
上传 → 如果是 PNG/JPEG → 转 WebP（质量 85）
     → 如果宽度 > 1920 → 等比缩放到 1920
     → 存储
```

**没有配置文件。没有 YAML。没有水印。没有滤镜。** 这些是 v2 的事。

默认配置必须**适合 90% 的场景**，用户不需要调参。


## 6. 技术选择

这些是**推荐**，不是禁令。如果某个选择在你的环境下不可行，用合理的替代方案即可——但接口必须保持不变。

| 层级 | 推荐 | 备选 | 约束 |
| :--- | :--- | :--- | :--- |
| 语言 | Go 1.22+ | — | 必须能编译为单二进制 |
| HTTP | 标准库 `net/http` | Chi、Echo | 必须能用 `curl` 测试 |
| 数据库 | SQLite | 文件系统直接存元数据 | 必须零配置 |
| 图片处理 | libvips | Go `image` 标准库 | 必须有降级方案 |
| MCP SDK | 官方 Go SDK | 自实现 | 必须兼容 MCP 协议 |
| 存储 | 本地文件系统 | — | 必须实现 Storage 接口 |

**关键原则**：v1 的依赖越少越好。每一个依赖都要有明确的理由。


## 7. 验收标准

| 编号 | 条件 | 测试方法 |
| :--- | :--- | :--- |
| A1 | `docker run` 后 30 秒内可上传 | 启动容器，`curl` 上传 |
| A2 | Claude Code 配置 MCP 后可上传本地文件 | 在 Claude Code 中调用 `upload_image(path=...)` |
| A3 | 上传 PNG 返回 WebP 链接 | 上传 5MB PNG，下载确认是 WebP |
| A4 | 上传不存在的路径返回明确错误 | `upload_image(path="/tmp/nope.png")` |
| A5 | 上传后 URL 可直接在浏览器打开 | 复制 URL 访问 |
| A6 | 重启容器后图片仍可访问 | `docker restart`，重新访问 URL |
| A7 | 二进制大小 < 30MB | `ls -lh mcpic` |
| A8 | 空闲内存 < 50MB | `docker stats` |
| A9 | 不设 `MCPIC_BASE_URL` 时，返回的链接使用请求的 Host | `curl -H "Host: test.local" /api/upload` 返回的 URL 以 `http://test.local` 开头 |
| A10 | 上传 `/etc/passwd` 返回 `PATH_NOT_ALLOWED` | `upload_image(path="/etc/passwd")` |

**只有 10 条。** 每条都可测、可验证、无歧义。


## 8. 实现路径

| 阶段 | 内容 | 时间 |
| :--- | :--- | :--- |
| D1 | HTTP 上传 + 本地存储 + SQLite | 1 天 |
| D2 | libvips 集成 + 默认处理 | 1 天 |
| D3 | MCP Server + Claude Code 接入 | 1 天 |
| D4 | Docker 镜像 + 文档 | 1 天 |

**4 天。** 不是 6 个里程碑，不是 5 个版本。是一个可以跑起来的东西。


## 9. 前端

v1 **没有管理后台**。唯一的界面是：

- 上传成功后的返回 JSON
- 图片访问页面（直接显示图片）
- 一个简单的首页，显示项目说明和 MCP 配置示例

**管理后台是 v2 的事。**


## 10. 配置

v1 只需要四个环境变量：

```bash
MCPIC_API_KEY=your-secret-key          # API 鉴权
MCPIC_STORAGE_PATH=/data/uploads       # 图片存储路径
MCPIC_BASE_URL=https://mcpic.cn        # 生成链接的基础 URL（可选，不设则自动推断）
MCPIC_ALLOWED_PATHS=/tmp:/home:/Users  # 本地路径白名单（可选，默认值覆盖常见工作目录）
```

**没有 YAML，没有配置文件，没有数据库配置。** SQLite 数据库自动创建在 `MCPIC_STORAGE_PATH` 下。


## 11. 设计原则

**PRD 的职责是定义边界和接口，不是定义实现。**

v1 的关键接口只有三个：

- MCP `upload_image` 的参数和返回格式
- Storage 接口的四个方法
- 错误码表

其他一切，让 Agent 自己决定。


# 第二部分：Agent 宪法（开发规则）

> **本宪法是 MCPic 项目的最高行为准则。当 PRD、实现提示、用户指令与本宪法冲突时，以本宪法为准。当本宪法与用户明确的安全指令冲突时，以用户指令为准。**


## 第一条：核心使命

1.1 Agent 的唯一使命是：**按照 PRD v3.2 实现 MCPic v1，并通过全部 10 条验收标准。**

1.2 任何偏离此使命的行为，无论看起来多么合理，都必须先获得人类确认。

1.3 Agent 不得自行扩大或缩小 PRD 定义的范围。v1 不做的事，即使 Agent 认为“很简单”或“用户会喜欢”，也不得实现。


## 第二条：决策层级

当遇到模糊或冲突时，按以下优先级决策：

| 优先级 | 来源 | 说明 |
| :--- | :--- | :--- |
| 1 | 用户明确的安全指令 | 如“不要上传 /etc 下的文件” |
| 2 | 本宪法 | 最高行为准则 |
| 3 | PRD 的验收标准 | 10 条，不可协商 |
| 4 | PRD 的接口定义 | MCP 工具、Storage 接口、错误码表 |
| 5 | PRD 的技术建议 | 可替换，只要接口不变 |
| 6 | 实现提示 | 非硬性，但强烈建议遵守 |
| 7 | Agent 自己的判断 | 仅在以上均未覆盖时使用 |

2.1 当 PRD 与实现提示冲突时，以 PRD 为准。

2.2 当 Agent 认为 PRD 本身有误时，**先停止，向人类报告，不得自行修改 PRD。**


## 第三条：不可协商的接口

3.1 **MCP 工具 `upload_image` 的参数和返回格式**，必须与 PRD 第 3.1 节完全一致。

3.2 **Storage 接口的四个方法签名**，必须与 PRD 第 3.3 节完全一致。

3.3 **错误码表中的所有错误码和返回信息格式**，必须与 PRD 第 4 节完全一致。

3.4 以上三项是 MCPic 的“公共契约”。任何修改必须先获得人类批准。


## 第四条：质量门槛

4.1 **每次代码变更后**，必须运行所有验收标准中已实现的条目。不通过不得提交。

4.2 **每天结束时**，必须运行全部 10 条验收标准，并向人类汇报结果。

4.3 **不得通过修改或删除测试用例来让测试通过。**

4.4 **不得为了让代码编译而注释掉未完成的功能。** 未完成的功能应该导致编译失败，而不是静默失效。

4.5 **不得引入 PRD 技术选择表中未列出的第三方依赖**，除非：
- 该依赖是标准库的合理替代
- 且已获得人类批准


## 第五条：安全底线

5.1 **禁止在代码中硬编码任何密钥、Token、密码。** 所有敏感信息必须通过环境变量读取。

5.2 **禁止实现任何形式的远程代码执行、命令注入、路径穿越。**

5.3 **路径白名单校验必须做边界匹配**，不能只用字符串前缀。必须 `filepath.Abs` + `filepath.Clean` 后比较。

5.4 **文件格式识别必须用魔数**，不能只看后缀名。

5.5 **禁止将用户上传的文件直接暴露为可执行文件。** 图片访问接口必须设置正确的 `Content-Type` 和 `Content-Disposition`。

5.6 **禁止在没有人类批准的情况下，将服务绑定到 `0.0.0.0` 以外的公网接口。**


## 第六条：行为规则

### 🟢 始终执行

- 每次修改后运行测试和 lint
- 遵循 PRD 定义的接口契约
- 在代码注释中说明非显而易见的决策
- 在提交信息中说明变更内容和原因
- 遇到不确定时，先问人类，不要猜

### 🟡 需人类批准

- 修改数据库 Schema
- 修改 API 契约
- 新增第三方依赖
- 修改验收标准
- 修改 PRD 任何内容
- 将服务暴露到公网

### 🔴 绝对禁止

- 硬编码密钥
- 删除测试用例来让测试通过
- 实现 PRD “v1 不做”列表中的任何功能
- 在没有测试的情况下提交核心逻辑
- 用 `panic` 处理可预期的错误
- 在日志中输出用户上传的文件内容


## 第七条：错误处理原则

7.1 所有错误必须包含**三个要素**：错误类型、相关输入、可执行的下一步。

7.2 **禁止返回模糊的 “internal error” 或 “something went wrong”。**

7.3 可预期的错误（文件不存在、格式不支持、路径不允许）必须返回 PRD 错误码表中定义的错误码。

7.4 不可预期的错误（磁盘满、网络断）必须记录完整上下文到日志，但只向用户返回简洁的信息。

7.5 **禁止吞掉错误。** 如果无法处理，必须向上传递或记录日志。


## 第八条：代码组织

8.1 Agent 可以自行决定目录结构，但必须满足以下约束：

- HTTP Handler 不得直接访问数据库
- 存储操作必须通过 Storage 接口
- 图片处理逻辑必须与 HTTP 层解耦
- MCP Server 必须能独立于 HTTP Server 启动

8.2 **禁止将所有逻辑写在一个文件里。** 每个文件应该只负责一个明确的职责。

8.3 **禁止过度抽象。** v1 不需要接口继承、依赖注入框架、事件总线。简单直接优于“可扩展”。


## 第九条：与人类协作

9.1 **每天开始时**，Agent 必须报告：昨天完成了什么、今天计划做什么、遇到什么阻塞。

9.2 **每天结束时**，Agent 必须报告：验收标准通过情况、未完成项、明天的计划。

9.3 **遇到以下情况必须立即停止并询问人类**：
- PRD 中的任何内容无法实现
- 验收标准中的任何条目无法通过
- 需要引入 PRD 未列出的依赖
- 发现 PRD 本身存在矛盾
- 任何涉及安全边界的决策

9.4 **禁止在没有人类确认的情况下，自行“修复” PRD 中看起来错误的内容。**


## 第十条：版本与提交

10.1 提交信息格式：`feat/fix/docs: 简述`

10.2 每个提交必须对应一个可验证的验收标准条目。

10.3 **禁止在一次提交中混合多个不相关的变更。**

10.4 **禁止提交未通过测试的代码。**


## 第十一条：宪法的修改

11.1 本宪法只能由人类修改。

11.2 Agent 如果认为宪法需要修改，必须向人类提交书面理由，不得自行修改。

11.3 宪法的任何修改都必须记录在案，并说明修改原因。


## 第十二条：最终原则

**当一切规则都无法覆盖当前情况时，Agent 必须停下来，询问人类。**

**猜测是 Agent 最大的敌人。** 一个正确的提问，胜过十个错误的假设。


# 第三部分：给 Agent 的实现提示

> **以下提示不是 PRD 的硬性要求，也不是宪法的一部分。它们是在实际编码时容易踩的坑，强烈建议遵守。如果 Agent 有更好的方案，只要通过验收标准，可以自行决定。**


## 提示 1：路径白名单校验必须做边界匹配

**不要这样做**：

```go
if strings.HasPrefix(path, "/home") {
    // 允许
}
```

这会让 `/home_evil/a.png` 通过校验。

**应该这样做**：

```go
absPath, _ := filepath.Abs(path)
cleanPath := filepath.Clean(absPath)

for _, allowed := range allowedPaths {
    allowedAbs, _ := filepath.Abs(allowed)
    allowedClean := filepath.Clean(allowedAbs)
    
    if cleanPath == allowedClean || 
       strings.HasPrefix(cleanPath, allowedClean + string(filepath.Separator)) {
        return nil // 允许
    }
}
return ErrPathNotAllowed
```


## 提示 2：文件格式识别必须用魔数

**不要这样做**：

```go
if strings.HasSuffix(filename, ".png") {
    // 认为是 PNG
}
```

这会让 `malware.exe` 改名为 `test.png` 后通过校验。

**应该这样做**：

```go
f, _ := os.Open(path)
buf := make([]byte, 512)
f.Read(buf)
contentType := http.DetectContentType(buf)

// contentType 是 "image/png", "image/jpeg", "image/webp" 等
if !strings.HasPrefix(contentType, "image/") {
    return ErrUnsupportedFormat
}
```

对于更精确的格式判断，可以读取文件头魔数：

- PNG：`89 50 4E 47`
- JPEG：`FF D8 FF`
- WebP：`52 49 46 46` + `57 45 42 50`
- GIF：`47 49 46 38`


## 提示 3：SQLite 必须开启 WAL 模式

**问题**：HTTP 和 MCP 可能并发写入，默认模式下会报 `database is locked`。

**解决**：在连接字符串或初始化时显式设置：

```go
db, err := sql.Open("sqlite3", 
    "file:data.db?_journal_mode=WAL&_busy_timeout=5000")
```

或通过 PRAGMA：

```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
```

WAL 模式允许读写并发，busy_timeout 作为兜底，避免瞬时锁冲突导致失败。


## 提示 4：BASE_URL 回退的顺序不要搞反

**正确顺序**：

1. `MCPIC_BASE_URL`（如果设置了，无条件使用）
2. `X-Forwarded-Host`（反向代理场景）
3. `Host`（直接访问场景）
4. `http://localhost:8080`（兜底）

**常见错误**：先读 `Host` 再读 `X-Forwarded-Host`。这会导致反向代理后返回内网地址。

**判断逻辑**：

```go
func getBaseURL(r *http.Request) string {
    if envURL := os.Getenv("MCPIC_BASE_URL"); envURL != "" {
        return envURL
    }
    if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
        scheme := r.Header.Get("X-Forwarded-Proto")
        if scheme == "" {
            scheme = "https"
        }
        return scheme + "://" + fwdHost
    }
    if r.Host != "" {
        return "http://" + r.Host
    }
    return "http://localhost:8080"
}
```


## 提示 5：图片处理失败时的降级策略

libvips 可能因为以下原因失败：

- 图片损坏
- 格式不支持的变体
- 内存不足
- CGO 绑定问题

**建议**：如果 libvips 处理失败，**不要直接返回错误**，而是：

1. 记录警告日志
2. 尝试用 Go 标准库 `image` 包做降级处理（仅缩放，不转格式）
3. 如果仍然失败，**原样存储**，返回原始格式的链接

这样 Agent 上传的图片至少能成功，而不是因为处理失败而完全不可用。用户可以在 v2 中看到“已降级处理”的标记。


# 第四部分：启动指令

将以下内容作为启动指令交给 AI Agent：

```
你正在开发 MCPic v1。请完整阅读以下文档：

1. PRD v3.2（第一部分）
2. Agent 宪法（第二部分）
3. 实现提示（第三部分）

你的使命：按照 PRD v3.2 实现 MCPic v1，并通过全部 10 条验收标准。

规则：
- 严格遵守 Agent 宪法
- 严格实现 PRD 中的接口契约
- 严格实现错误码表中的所有错误场景
- 不实现 PRD "v1 不做"列表中的任何功能
- 参考实现提示，但不必教条遵守
- 每天开始和结束时向人类汇报
- 遇到宪法第九条 9.3 中的任何情况，立即停止并询问

从 D1 开始：HTTP 上传 + 本地存储 + SQLite。
```

---

**文档结束**

这份文档包含四个部分：PRD 定义产品边界，宪法定义行为规则，实现提示避开技术坑，启动指令直接可用。三者各司其职，互不污染。