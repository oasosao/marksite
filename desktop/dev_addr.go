//go:build !production

package main

import "os"

// desktopListenAddr 开发版固定监听 8080，便于 Vite 把 /api、/media 代理到桌面端内部服务。
// 需要避开端口冲突时可用 MARKSITE_DEV_ADDR 覆盖。
func desktopListenAddr() string {
	if addr := os.Getenv("MARKSITE_DEV_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:8080"
}
