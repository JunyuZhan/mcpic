// Package httpapi 实现 PRD 3.2 的 HTTP 接口。
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mcpic/mcpic/internal/apperr"
	"github.com/mcpic/mcpic/internal/meta"
)

// errorResponse 是统一错误响应体。
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeAppError 把应用错误映射为 HTTP 状态码与 JSON（PRD 第 4 节错误码表）。
func writeAppError(w http.ResponseWriter, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		writeJSON(w, statusFor(ae.Code), errorResponse{Error: ae.Code, Message: ae.Message})
		return
	}
	if errors.Is(err, meta.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{
			Error:   "IMAGE_NOT_FOUND",
			Message: "图片不存在，请确认链接是否完整",
		})
		return
	}
	// 不允许模糊的 internal error（宪法 7.2）：这里必然已经不是可预期错误，
	// 调用方负责记录上下文日志。
	writeJSON(w, http.StatusInternalServerError, errorResponse{
		Error:   "STORAGE_ERROR",
		Message: "服务内部错误：请联系管理员并附上时间点",
	})
}

func statusFor(code string) int {
	switch code {
	case apperr.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperr.CodeFileTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperr.CodeStorageError:
		return http.StatusInternalServerError
	case apperr.CodeFileNotFound,
		apperr.CodeUnsupportedFormat,
		apperr.CodeProcessError,
		apperr.CodePathNotAllowed:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
