package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"marksite/internal/bootstrap"
	"marksite/internal/db"
)

func main() {
	// 子命令分流：在 flag.Parse 之前检测
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		subCmd := os.Args[1]
		// 重置 flag 集合并解析子命令后面的参数
		flag.CommandLine = flag.NewFlagSet(subCmd, flag.ExitOnError)
		workDirFlag := flag.CommandLine.String("workdir", "./workspace", "工作目录")
		forceFlag := flag.CommandLine.Bool("force", false, "强制覆盖已有内容")
		_ = flag.CommandLine.Parse(os.Args[2:]) // ExitOnError 遇到错误自动 os.Exit，不会返回 error
		if subCmd == "seed" {
			os.Exit(runSeed(*workDirFlag, *forceFlag))
		}
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n可用: seed\n", subCmd)
		os.Exit(1)
		return
	}

	addrFlag := flag.String("addr", "", "监听地址覆盖（格式 host:port）；留空则使用数据库配置或默认 127.0.0.1:8080")
	workDirFlag := flag.String("workdir", "./workspace", "工作目录（其中 data/ 存源数据，output/ 存生成的静态站点）")
	openFlag := flag.Bool("open", false, "启动后自动打开浏览器（默认不打开）")
	flag.Parse()

	rt, err := bootstrap.Init(*workDirFlag)
	if err != nil {
		slog.Error("初始化失败", "err", err)
		os.Exit(1)
	}
	defer rt.Close()
	app := rt.App
	paths := rt.Paths
	ctx := context.Background()

	// 配置只读一次，监听地址与"自动打开浏览器"共用
	cfg, cfgErr := app.GetAppConfig(ctx)
	addr := "127.0.0.1:8080"
	if *addrFlag != "" {
		addr = *addrFlag
	} else if cfgErr == nil && cfg.ListenAddr != "" {
		addr = cfg.ListenAddr
	}
	openBrowser := *openFlag || (cfgErr == nil && cfg.AutoOpenBrowser == 1)

	ln, err := rt.Listen(addr)
	if err != nil {
		slog.Error("监听失败", "addr", addr, "err", err)
		os.Exit(1)
	}
	srv := &http.Server{Handler: rt.Handler()}

	// 自动备份调度：每 10 分钟检查一次
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rt.StartBackupScheduler(stopCtx)

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务退出", "err", err)
			stop()
		}
	}()

	url := "http://" + browserHost(addr) + "/"
	slog.Info("MarkSite 已启动", "url", url, "db", paths.DBPath)
	if openBrowser {
		go func() {
			time.Sleep(600 * time.Millisecond)
			openInBrowser(url)
		}()
	}

	<-stopCtx.Done()
	slog.Info("正在关闭...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// browserHost 将监听地址转换为可访问的浏览器地址
func browserHost(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return host + ":" + port
}

// openInBrowser 自动打开系统默认浏览器
func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		slog.Warn("自动打开浏览器失败", "err", err, "url", url)
	}
	if cmd.Process != nil {
		go func() { _ = cmd.Wait() }()
	}
}

// runSeed seed 子命令：初始化 demo 数据
func runSeed(workDir string, force bool) int {
	paths := bootstrap.PathsFor(workDir)
	if err := db.EnsureDataDir(paths.DataDir, paths.FTSDir); err != nil {
		fmt.Fprintln(os.Stderr, "创建目录失败:", err)
		return 1
	}
	conn, err := db.Open(paths.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开数据库失败:", err)
		return 1
	}
	defer func() { _ = conn.Close() }()

	var count int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&count)
	if count > 0 && !force {
		fmt.Fprintf(os.Stderr, "数据库已有 %d 篇文章，加 -force 强制覆盖\n", count)
		return 1
	}

	if err := db.SeedDemo(conn); err != nil {
		fmt.Fprintln(os.Stderr, "Seed 失败:", err)
		return 1
	}

	fmt.Println("✅ Demo 数据已灌入:")
	fmt.Println("   目录 9 / 文章 18 / 页面 6 / 标签 8")
	fmt.Printf("   数据库: %s\n", paths.DBPath)
	fmt.Println("\n启动服务:  ./marksite -workdir", workDir)
	fmt.Println("启动后记得在后台点 '重建索引' 刷新全文检索")
	return 0
}
