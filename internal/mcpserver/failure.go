package mcpserver

import (
	"errors"
	"fmt"

	"github.com/mcpic/mcpic/internal/apperr"
)

// source 是三选一来源的判定结果。
type source int

const (
	sourceNone source = iota
	sourcePath
	sourceURL
	sourceData
)

// pickSource 校验 path / url / data 必须且只能提供一个（PRD 3.1）。
func pickSource(args uploadArgs) (source, error) {
	n := 0
	var s source
	if args.Path != "" {
		n++
		s = sourcePath
	}
	if args.URL != "" {
		n++
		s = sourceURL
	}
	if args.Data != "" {
		n++
		s = sourceData
	}
	if n == 0 {
		return sourceNone, apperr.BadRequest(
			"必须提供 path、url、data 三者之一：path 为本地绝对路径，url 为远程图片地址，data 为 base64 数据")
	}
	if n > 1 {
		return sourceNone, apperr.BadRequest(
			fmt.Sprintf("path、url、data 只能提供一个，当前提供了 %d 个", n))
	}
	return s, nil
}

// describeSource 生成失败文案中的“相关输入”行。
func describeSource(args uploadArgs) string {
	switch {
	case args.Path != "":
		return "路径: " + args.Path
	case args.URL != "":
		return "URL: " + args.URL
	case args.Data != "":
		return fmt.Sprintf("数据: base64，%d 个字符", len(args.Data))
	default:
		return "参数: 未提供 path / url / data"
	}
}

// failureTable 是错误码 →（错误类型，可执行的下一步）映射。
// 错误消息的三要素（错误类型、相关输入、下一步）由 renderFailure 拼装。
var failureTable = map[string]struct{ kind, next string }{
	apperr.CodeFileNotFound:      {"文件不存在", "请检查路径是否正确。"},
	apperr.CodePathNotAllowed:    {"路径不在允许范围内", "请改用 /tmp、/home、/Users 下的路径，或调整 MCPIC_ALLOWED_PATHS。"},
	apperr.CodeUnsupportedFormat: {"格式不支持", "请确认文件是 jpg/png/webp/gif 格式。"},
	apperr.CodeFileTooLarge:      {"文件超过大小限制", "请把图片压缩到 10MB 以内后重试。"},
	apperr.CodeStorageError:      {"存储失败", "请检查磁盘空间，并确认 MCPIC_STORAGE_PATH 可写。"},
	apperr.CodeProcessError:      {"图片处理失败", "请确认图片完整未损坏，或改用 format=original 重试。"},
	apperr.CodeUnauthorized:      {"API Key 无效", "请检查服务端 MCPIC_API_KEY 配置是否一致。"},
	apperr.CodeBadRequest:        {"参数错误", "请检查参数格式后重试。"},
	apperr.CodeDownloadFail:      {"远程图片下载失败", "请确认 URL 公网可达，或改用 path / data 方式上传。"},
}

// renderFailure 按 PRD 3.1 的三段式输出失败文案：
//
//	上传失败：<错误类型>
//	<相关输入>
//	<可执行的下一步>
func renderFailure(err error, inputLine string) string {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		// 未归类错误：给出通用但可执行的指引，不返回模糊的 internal error。
		return fmt.Sprintf("上传失败：未预期的错误\n%s\n请把上面的完整信息反馈给维护者。", inputLine)
	}
	entry, ok := failureTable[ae.Code]
	if !ok {
		entry = struct{ kind, next string }{"错误（" + ae.Code + "）", "请把上面的完整信息反馈给维护者。"}
	}
	line := "上传失败：" + entry.kind
	// 参数错误的具体原因在 Message 里，拼到第一行避免丢失细节。
	if ae.Code == apperr.CodeBadRequest && ae.Message != "" {
		line += "：" + ae.Message
	}
	return fmt.Sprintf("%s\n%s\n%s", line, inputLine, entry.next)
}
