package pkg

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// 统一业务错误码
const (
	CodeOK         = 0
	CodeBadRequest = 40001
	CodeNotFound   = 40401
	CodeConflict   = 40901
	CodeBusy       = 42901
	CodeInternal   = 50001
)

// AppError 业务错误，handler 层统一转换为 JSON 响应
type AppError struct {
	Status int
	Code   int
	Msg    string
}

func (e *AppError) Error() string { return e.Msg }

func BadRequest(msg string) *AppError {
	return &AppError{Status: http.StatusBadRequest, Code: CodeBadRequest, Msg: msg}
}

func NotFound(msg string) *AppError {
	return &AppError{Status: http.StatusNotFound, Code: CodeNotFound, Msg: msg}
}

func Conflict(msg string) *AppError {
	return &AppError{Status: http.StatusConflict, Code: CodeConflict, Msg: msg}
}

func Busy(msg string) *AppError {
	return &AppError{Status: http.StatusTooManyRequests, Code: CodeBusy, Msg: msg}
}

func Internal(msg string) *AppError {
	return &AppError{Status: http.StatusInternalServerError, Code: CodeInternal, Msg: msg}
}

// OK 返回成功响应
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, map[string]any{"code": CodeOK, "message": "ok", "data": data})
}

// WriteErr 输出错误响应
func WriteErr(w http.ResponseWriter, r *http.Request, err error) {
	var ae *AppError
	if errors.As(err, &ae) {
		if ae.Code >= 50000 {
			slog.Error("API error", "path", routePattern(r), "status", ae.Status, "code", ae.Code, "err", err)
		}
		JSON(w, ae.Status, map[string]any{"code": ae.Code, "message": ae.Msg, "data": nil})
		return
	}
	slog.Error("API internal error", "path", routePattern(r), "err", err)
	JSON(w, http.StatusInternalServerError, map[string]any{
		"code": CodeInternal, "message": "服务器内部错误", "data": nil,
	})
}

// JSON 输出 JSON 响应
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("写 JSON 响应失败", "err", err)
	}
}

// Data 输出原始字节响应
func Data(w http.ResponseWriter, status int, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		slog.Warn("写响应体失败", "err", err)
	}
}

// BindJSON 绑定并校验 JSON 请求体（限制 1MB 防止恶意超大 body）
func BindJSON(w http.ResponseWriter, r *http.Request, obj any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
		WriteErr(w, r, BadRequest("请求参数格式错误: "+err.Error()))
		return false
	}
	return true
}

// routePattern 取已匹配的路由模式，未匹配时退回实际路径
func routePattern(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.URL.Path
}
