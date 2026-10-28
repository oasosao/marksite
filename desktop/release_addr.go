//go:build production

package main

// desktopListenAddr 发布版由系统分配空闲端口，避免与其他程序冲突
func desktopListenAddr() string {
	return "127.0.0.1:0"
}
