package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"marksite/internal/pkg"
)

// Middleware HTTP 中间件（包在 Handler 外层）
type Middleware func(http.Handler) http.Handler

// Chain 按顺序组合中间件，第一个参数在最外层
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// Recovery panic 恢复
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("请求处理 panic", "path", r.URL.Path, "err", err)
				pkg.WriteErr(w, r, pkg.Internal("服务器内部错误"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// NoCache 禁止缓存 API 响应：桌面端 WebView 会复用同 URL 的 GET 结果，导致界面上看不到刚改的数据
func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}

// AccessLog 访问日志
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"ms", time.Since(start).Milliseconds(),
			"ip", pkg.ClientIP(r),
			"ua", r.Header.Get("User-Agent"),
		)
	})
}

// statusWriter 记录响应状态码，供访问日志使用
type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap 供 http.ResponseController 使用
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
