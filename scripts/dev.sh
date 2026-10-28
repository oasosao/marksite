#!/usr/bin/env bash
# 浏览器开发模式：同时启动 Go 后端与 Vite 前端（前端热更新）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="${MARKSITE_WORKDIR:-$ROOT/.dev/site}"
ADDR="${MARKSITE_ADDR:-127.0.0.1:8080}"
BIN="${TMPDIR:-/tmp}/marksite-dev"

pids=()
cleanup() {
  trap - EXIT INT TERM
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

if [ ! -d "$ROOT/web/node_modules" ]; then
  echo "安装前端依赖..."
  (cd "$ROOT/web" && npm install)
fi

echo "编译后端..."
(cd "$ROOT" && go build -o "$BIN" ./cmd/marksite)

echo "后端  http://$ADDR  工作目录 $WORKDIR"
"$BIN" -workdir "$WORKDIR" -addr "$ADDR" &
pids+=($!)

echo "前端  http://localhost:5173"
(cd "$ROOT/web" && npm run dev) &
pids+=($!)

wait
