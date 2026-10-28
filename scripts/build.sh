#!/usr/bin/env bash
# MarkSite 单二进制构建脚本：前端构建 -> embed -> 无 CGO 编译
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> 安装前端依赖"
cd "$ROOT_DIR/web"
npm ci

echo "==> 准备编辑器静态资源"
npm run prepare-vditor

echo "==> 构建前端（类型检查 + 打包到 internal/webui/dist）"
npm run build

echo "==> 编译 Go 单二进制（CGO_ENABLED=0）"
cd "$ROOT_DIR"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o marksite ./cmd/marksite

echo "==> 构建完成：$ROOT_DIR/marksite"
ls -lh "$ROOT_DIR/marksite"
