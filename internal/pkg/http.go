package pkg

import (
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// maxMultipartMemory 上传表单在内存中的阈值，超出部分落临时文件
const maxMultipartMemory = 32 << 20

// maxMultipartTotal 上传表单总大小上限（包含所有文件），防恶意超大 body 耗尽磁盘
const maxMultipartTotal = 200 << 20

// Query 读取查询参数
func Query(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// QueryDefault 读取查询参数，缺省时返回默认值
func QueryDefault(r *http.Request, key, def string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return def
}

// PostForm 读取表单字段（自动解析 multipart 表单，限制总大小 200MB）
func PostForm(w http.ResponseWriter, r *http.Request, key string) string {
	if r.PostForm == nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxMultipartTotal)
		_ = r.ParseMultipartForm(maxMultipartMemory)
	}
	if r.PostForm == nil {
		return ""
	}
	return r.PostForm.Get(key)
}

// FormFile 解析并返回上传文件（限制 multipart 总大小 200MB）
func FormFile(w http.ResponseWriter, r *http.Request, field string) (*multipart.FileHeader, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartTotal)
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return nil, err
	}
	_, fh, err := r.FormFile(field)
	return fh, err
}

// Attachment 以附件方式下载文件，兼容中文文件名（RFC 5987）
func Attachment(w http.ResponseWriter, r *http.Request, path, filename string) {
	encoded := url.PathEscape(filename)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+encoded+"\"; filename*=UTF-8''"+encoded)
	http.ServeFile(w, r, path)
}

// ClientIP 解析客户端地址，优先取反向代理头
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
