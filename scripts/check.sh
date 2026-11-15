#!/usr/bin/env bash
# 门禁（wt land 合入前与 ci.yml 的 format job 都跑它）：只含能在 Linux 上跑的检查。
# Windows 部分（scripts/test.ps1 与 UI 验收）见 CLAUDE.md。
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

uv lock --check --script scripts/make-icon.py
bash scripts/format.sh --check
bash scripts/format.sh --lint
