#!/usr/bin/env bash
# 门禁（wt land 合入前与 ci.yml 的 format job 都跑它）：只含能在 Linux 上跑的检查。
# Windows 部分（scripts/test.ps1 与 UI 验收）见 CLAUDE.md。
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck source=scripts/gatelib.sh
source scripts/gatelib.sh
# 仓内不放 Python 文件，有就列出并拒绝。
gate_python_allowlist

bash scripts/format.sh --check
bash scripts/format.sh --lint
