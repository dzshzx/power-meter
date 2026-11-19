#!/usr/bin/env bash
# 门禁（wt land 合入前与 ci.yml 都跑它）：只含能在 Linux 上跑的检查。
# Windows 部分（Windows 上的 go test 与 tools/release 的烟测、安装实测）见 CLAUDE.md。
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=scripts/gatelib.sh
source scripts/gatelib.sh
# 仓内不放 Python 文件，有就列出并拒绝。
gate_python_allowlist

bash scripts/format.sh --check
bash scripts/format.sh --lint

# Go：版本只写在 go.mod 的 go 行；之后都用这份工具链的绝对路径。
gate_go_version .
GO="$(go env GOROOT)/bin/go"
# 门禁不取 MiSans：没有字体也能编译和测试，结果只取决于提交；字体只在 Windows 验证与发版里取。
# 程序只发 windows/amd64，Windows 专属的包（带 windows 构建约束）在 Linux 上也要类型检查和静态检查，
# 所以 vet 与 staticcheck 按 linux 与 windows 各跑一遍；它们的测试在 Windows 上跑。
"$GO" vet ./...
GOOS=windows GOARCH=amd64 "$GO" vet ./...
# 规则集在仓根 staticcheck.conf。staticcheck 先按本机平台装到临时目录，再分别按两个平台分析。
tools=$(mktemp -d)
trap 'rm -rf "$tools"' EXIT
GOBIN="$tools" GOPROXY="file://$("$GO" env GOMODCACHE)/cache/download,$("$GO" env GOPROXY)" "$GO" install "$STATICCHECK"
"$tools/staticcheck" ./...
GOOS=windows GOARCH=amd64 "$tools/staticcheck" ./...
"$GO" test -count=1 -vet=off ./...
# gocognit 只读源码，有函数超线时列出并非零退出；复杂度不认代码里的豁免标记，这里直接查标记。
gate_go_run "$GOCOGNIT" -over "$MAX_COGNITIVE" .
gate_no_marker 'gocognit:ignore' '*.go'
echo "check: ok"
