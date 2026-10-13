// Package apperr 定义 PRD 第 4 节的错误码表。
// 每个错误必须包含三要素：错误类型、相关输入、可执行的下一步（宪法第七条）。
package apperr

import "fmt"

// 错误码（PRD 第 4 节，不可协商）。
const (
	CodeFileNotFound      = "FILE_NOT_FOUND"
	CodeUnsupportedFormat = "UNSUPPORTED_FORMAT"
	CodeFileTooLarge      = "FILE_TOO_LARGE"
	CodeStorageError      = "STORAGE_ERROR"
	CodeProcessError      = "PROCESS_ERROR"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodePathNotAllowed    = "PATH_NOT_ALLOWED"
)

// Error 是携带错误码的应用错误。
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New 构造一个错误码错误。
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// FileNotFound 路径不存在。
func FileNotFound(path string) *Error {
	return New(CodeFileNotFound, fmt.Sprintf("路径 %s 不存在，请检查路径", path))
}

// UnsupportedFormat 文件不是受支持的图片格式。ext 含前导点，如 ".bmp"。
func UnsupportedFormat(ext string) *Error {
	if ext == "" {
		return New(CodeUnsupportedFormat, fmt.Sprintf("不支持的文件格式，支持：%s", "jpg/png/webp/gif"))
	}
	return New(CodeUnsupportedFormat, fmt.Sprintf("不支持 %s 格式，支持：%s", ext, "jpg/png/webp/gif"))
}

// FileTooLarge 超过大小限制。size 已格式化（如 "15MB"）。
func FileTooLarge(size string) *Error {
	return New(CodeFileTooLarge, fmt.Sprintf("文件 %s 超过限制 10MB", size))
}

// StorageError 存储写入失败。
func StorageError(cause error) *Error {
	return New(CodeStorageError, fmt.Sprintf("本地存储写入失败：%v", cause))
}

// ProcessError 图片处理失败。
func ProcessError() *Error {
	return New(CodeProcessError, "图片处理失败：图片可能已损坏")
}

// Unauthorized API Key 校验失败。
func Unauthorized() *Error {
	return New(CodeUnauthorized, "API Key 无效或缺失")
}

// PathNotAllowed 路径不在白名单内。allowed 是允许的路径前缀列表。
func PathNotAllowed(path string, allowed []string) *Error {
	return New(CodePathNotAllowed,
		fmt.Sprintf("路径 %s 不在允许范围内，允许的路径前缀：%v", path, allowed))
}
