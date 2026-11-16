#!/usr/bin/env bash
# 门禁（wt land 合入前与 ci.yml 的 format job 都跑它）：只含能在 Linux 上跑的检查。
# Windows 部分（scripts/test.ps1 与 UI 验收）见 CLAUDE.md。
# 用法：bash scripts/check.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# 仓内的 Python 文件只许是 scripts/python-allowlist.txt 列出的（# 起注释，以 / 结尾的行是目录前缀）。
mapfile -t allowed < <(sed -e 's/#.*//' -e 's/[[:space:]]*$//' scripts/python-allowlist.txt | grep -v '^$' || true)
stray=0
while IFS= read -r -d '' f; do
  ok=0
  for a in "${allowed[@]}"; do
    if [[ "$f" == "$a" || ("$a" == */ && "$f" == "$a"*) ]]; then
      ok=1
      break
    fi
  done
  if [ "$ok" -eq 0 ]; then
    echo "check: $f 不在 scripts/python-allowlist.txt 里" >&2
    stray=1
  fi
done < <(git ls-files -z -- '*.py')
[ "$stray" -eq 0 ]

bash scripts/format.sh --check
bash scripts/format.sh --lint
