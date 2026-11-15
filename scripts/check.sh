#!/usr/bin/env bash
# 合入门禁：wt land 在合入前运行本脚本，只含能在 Linux 上跑的检查：
# make-icon.py 的脚本锁、format.sh --check（格式）与 format.sh --lint（lint、复杂度、行数）。
# 手动触发的 Windows validation（.github/workflows/ci.yml）在 ubuntu-latest 上跑后两条，
# 在 windows-latest 上跑脚本锁检查与 scripts/test.ps1。
# Windows 部分（scripts/test.ps1 与 docs/testing/ui-localization.md 的 UI 验收）不能在 WSL 跑，
# 按 CLAUDE.md 在合入前于 Windows 本地 checkout 手动运行。
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

uv lock --check --script scripts/make-icon.py
bash scripts/format.sh --check
bash scripts/format.sh --lint
