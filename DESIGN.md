# MCPic DESIGN.md

> 设计规范，供 AI Agent 与开发者在修改首页 / 图片页时无条件遵守。
> 来源：[Refero Styles — Linear](https://styles.refero.design/style/90ce5883-bb24-4466-93f7-801cd617b0d1)（"midnight precision instrument"），按本项目约束裁剪。
> 本项目约束（PRD 第 9 节）：**仅原生 HTML + 内联 CSS + 极少量原生 JS，禁止任何前端构建工具链与运行时框架。**

## 1. 设计原则

近黑画布 + 纸白文字 + 单一酸性黄绿色强调色。暗色是基底而非主题：字重克制（≤590），边框是 0.5–1px 发丝线而非阴影，圆角只有三档，装饰为零 —— 让内容本身成为唯一的视觉纹理。

## 2. 色彩 Tokens

| Token | 值 | 用途 |
| :--- | :--- | :--- |
| `--void` | `#08090a` | 页面画布，全幅背景 |
| `--carbon` | `#0f1011` | 卡片表面、导航条 |
| `--obsidian` | `#161718` | 抬升面板、代码块底 |
| `--graphite` | `#23252a` | 发丝边框、分隔线、幽灵按钮描边 |
| `--smoke` | `#383b3f` | 高对比边框、区块分隔 |
| `--ash` | `#62666d` | 弱化正文、次级元数据 |
| `--fog` | `#8a8f98` | 三级文字、占位符、图标 |
| `--mist` | `#d0d6e0` | 次级标题、暗底按钮文字 |
| `--bone` | `#e5e5e6` | 近白填充、高对比按钮文字 |
| `--paper` | `#ffffff` | 主标题、最大对比强调 |
| `--acid-lime` | `#e4f222` | **唯一**主操作色（复制 Markdown 按钮、激活态） |
| `--pulse-green` | `#27a644` | 成功状态点缀（badge 描边） |
| `--coral-red` | `#eb5757` | 错误状态点缀（badge 描边） |

**规则**：每个视图最多一个酸性黄绿填充元素；正文只用灰阶（`--mist`/`--fog`/`--ash`），禁止彩色正文。

## 3. 字体

单二进制不加载任何外部字体文件，用系统栈近似 Linear 的 Inter：

```css
--font-sans: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
--font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
```

| 角色 | 字号 | 字重 | 行高 | 字距 |
| :--- | :--- | :--- | :--- | :--- |
| display | 56–72px | 510→600 以下 | 1.0 | -0.022em |
| heading | 32–48px | 510 | 1.13 | -0.012em |
| subheading | 24px | 400 | 1.33 | -0.012em |
| body | 16px | 400 | 1.5 | default |
| body-sm | 15px | 400 | 1.6 | -0.011em |
| caption | 13px | 400 | 1.2 | default |
| code | 13–14px | 400 | 1.6 | -0.013em |

**禁止**：700+ 粗体、彩色正文、衬线标题。

## 4. 间距与形状

- 基数 4px；间距梯：4 / 8 / 12 / 16 / 24 / 32 / 48 / 96
- 页面最大宽度 1100px，水平内边距 24px
- 卡片内边距 24px，元素间距 8–12px，区块间距 64–96px
- 圆角三档且仅三档：**按钮 6px、卡片 12px、pill 9999px**
- 分隔靠发丝边框 `1px solid var(--graphite)`，**不用投影**（唯一例外：代码块可用 1px inset 边框）

## 5. 组件

### 主操作按钮（酸性黄绿）
`background: var(--acid-lime); color: var(--void); border-radius: 6px; padding: 10px 16px; font-size: 14px; font-weight: 550; letter-spacing: -0.011em;` 每屏仅一个，用于"复制 Markdown"。

### 幽灵按钮
`background: transparent; border: 1px solid var(--graphite); color: var(--mist); border-radius: 6px; padding: 8px 12px; font-size: 13px;` hover 时边框提亮到 `--smoke`。

### 卡片
`background: var(--carbon); border-radius: 12px; box-shadow: inset 0 0 0 1px var(--graphite); padding: 24px;`

### 代码块
`background: var(--obsidian); border: 1px solid var(--graphite); border-radius: 6px; padding: 16px; font-family: var(--font-mono); font-size: 13px; color: var(--mist); overflow-x: auto;`

### Badge / Pill
`background: rgba(255,255,255,0.05); color: var(--fog); border-radius: 9999px; padding: 4px 12px; font-size: 12px;` 成功/错误变体改用 `--pulse-green` / `--coral-red` 作描边或文字。

### 元数据行
标签 `--ash`、值 `--mist`，等宽字体渲染技术值（尺寸、格式、key）。

## 6. 页面规范

### 首页 `/`
1. 顶栏：`MCPic` 字标（16px/510/`--paper`）+ 中部导航「上传 · 画廊」（14px `--fog`，hover `--paper`）+ 右侧 pill 徽标 `self-hosted image bed`
2. Hero：单行大标题（48–56px，`-0.022em`）+ 一句副文（16px `--fog`）
3. 三步快速开始（卡片栅格，最多 3 列）：Docker 启动 → curl 上传 → 拿链接
4. **MCP 配置示例**（代码块 + 幽灵复制按钮）—— 必须包含，Claude Code JSON 可直接复制
5. API 速查表（表格：方法/路径/说明）
6. 页脚：AGPL-3.0 · PRD v3.3 · GitHub 链接位

### 图片页（`GET /i/:key`，Accept 含 text/html 时）
1. 顶栏同首页（pill 位显示图片 key）
2. 居中图片预览卡（`--carbon` 底、12px 圆角、图片 contain、发丝边框）
3. 元数据行：格式 / 宽×高 / 大小 / key（等宽）
4. 操作区：**复制 Markdown**（酸性黄绿主按钮）+ 复制 URL（幽灵按钮）
5. 图片原始字节仍由 `?raw=1` 提供，`<img src>` 指向它

### 上传页（`GET /upload`）
1. 顶栏同首页，「上传」为激活态（`--paper`）
2. 标题「上传图片」+ 副文（粘贴快捷键用等宽 `kbd`）
3. API Key 行：等宽密码输入框 + 小尺寸酸性黄绿「保存」按钮；下方 12px 说明（localStorage）
4. **拖拽区**：`--carbon` 底、12px 圆角、`1px dashed --smoke` 发丝虚线；hover/dragover 边框转 `--acid-lime`、底色 `#111207`；整块可点触发文件选择
5. 状态行（22px 占位）：成功 `--acid-lime`、失败 `--coral-red`，错误必须展示服务端三要素消息
6. 结果卡（上传成功后出现）：96px 方形缩略图 + 等宽元数据（尺寸/格式/大小）+ **四行复制项**（URL / Markdown / HTML / BBCode）：76px `--ash` 标签 + 等宽值框 + 幽灵「复制」按钮（复制后 1.2s 变「已复制」）

### 画廊页（`GET /gallery`）
1. 顶栏同首页，「画廊」为激活态
2. **鉴权门禁（D4.7）**：页面为无数据壳；加载时检查 localStorage API Key，无 Key 强制跳转 `/upload`；有 Key 则 `GET /api/gallery`（带 `X-API-Key`）拉取列表渲染，401 时清 Key 并回 `/upload`
3. 头部行：标题「画廊」+ 右侧等宽计数 `N 张图片`
4. **网格**：`repeat(auto-fill, minmax(240px, 1fr))`，卡片 `--carbon` 底 12px 圆角发丝边框，hover 边框提亮 `--smoke`
5. 卡片：4:3 图片区（`?raw=1`，cover，lazy 加载）→ 徽标行（格式 / 尺寸 / 大小 / 时间，pill 等宽）→ key 路径行（等宽、省略号）→ 操作组：URL / Markdown / HTML / BBCode 四个幽灵按钮 + 右侧 `--coral-red`「删除」（间距隔开）
6. 删除流程：浏览器 confirm → `DELETE /api/images/{key}` 带 `X-API-Key` → 成功卡片移除并刷新计数，计数归零时刷新页面；401 提示去上传页填 Key
7. **空状态**：单张居中卡（64px 上下留白）：「还没有图片」+ 一句引导 + 酸性黄绿「去上传」链接按钮

## 7. Do / Don't

- ✅ 发丝边框分层，❌ 投影分层
- ✅ 每屏一个酸性黄绿主操作，❌ 多个彩色 CTA
- ✅ 灰阶正文 + 等宽技术值，❌ 彩色正文
- ✅ 6px/12px/pill 三档圆角，❌ 16px+ 大圆角
- ✅ 原生 HTML/CSS/JS 内联，❌ React/Vue/Vite/CDN 框架/外部字体文件
