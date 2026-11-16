#!/usr/bin/env bash
# 门禁（wt land 合入前与 ci.yml 的 format job 都跑它）：只含能在 Linux 上跑的检查。
# Windows 部分（scripts/test.ps1 与 UI 验收）见 CLAUDE.md。
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# 仓内不放 Python 文件，有就列出并拒绝。
py=$(git ls-files '*.py')
if [ -n "$py" ]; then
  printf 'check: 仓内不放 Python 文件：\n%s\n' "$py" >&2
  exit 1
fi

bash scripts/format.sh --check
bash scripts/format.sh --lint
