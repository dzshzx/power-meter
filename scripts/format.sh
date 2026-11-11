#!/usr/bin/env bash
# 统一格式化与检查入口：Python 用 ruff，JS/TS 与 Markdown/YAML/JSON 用 prettier，Shell 用 shfmt 与 shellcheck，
# C# 用 lizard 查函数复杂度。
# 用法：scripts/format.sh [--check | --lint]
#   无参数：就地格式化本仓跟踪的文件与未被忽略的新文件；--check：只检查格式，不一致时列出文件并非零退出；
#   --lint：跑 ruff check、Python 与 C# 函数复杂度上限、shellcheck，并检查代码文件不超过 MAX_LINES 行，有问题时非零退出。
#   后两种 CI 用。
# 工具版本只在下方常量固定；各工具配置：.prettierrc.json/.prettierignore、.editorconfig（shfmt），ruff 用默认风格。
set -euo pipefail

RUFF_VERSION=0.17.3
PRETTIER_VERSION=3.9.10
SHFMT_VERSION=3.14.1
# shfmt 经 PyPI 包 shfmt-py 分发，该版本内置上面的 shfmt。
SHFMT_PY_VERSION=4.2.0
SHELLCHECK_VERSION=0.11.0
# PyPI 包 shellcheck-py 分发 shellcheck，该版本内置上面的 shellcheck。
SHELLCHECK_PY_VERSION=0.11.0.1
LIZARD_VERSION=1.26.0
# 单个 Python 或 C# 函数的圈复杂度上限与单个代码文件的行数上限。
MAX_COMPLEXITY=15
MAX_LINES=2000
# 字节必须保持不变的文件（git pathspec），三种语言共用；与 .prettierignore 保持一致。
# third_party/：上游许可证与通知，含哈希钉住的文件，NOTICE.md 按字节嵌入 EXE 并与发布件比对。
EXCLUDE=(':(exclude)third_party/**' ':(exclude)**/*.lock' ':(exclude)**/dist/**')

CHECK=0
LINT=0
case "${1:-}" in
  "") ;;
  --check) CHECK=1 ;;
  --lint) LINT=1 ;;
  -h | --help)
    sed -n '2,8p' "$0"
    exit 0
    ;;
  *)
    echo "未知参数：$1" >&2
    exit 2
    ;;
esac
[ $# -le 1 ] || {
  echo "参数过多" >&2
  exit 2
}

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

files() {
  local f
  git ls-files -z --cached --others --exclude-standard -- "$@" "${EXCLUDE[@]}" | while IFS= read -r -d '' f; do
    [ -f "$f" ] || continue
    printf '%s\0' "$f"
  done
}

ruff() {
  if command -v uvx >/dev/null 2>&1; then
    uvx -q "ruff==$RUFF_VERSION" "$@"
  else
    pipx run --spec "ruff==$RUFF_VERSION" ruff "$@"
  fi
}

shfmt() {
  # 本机已装同版本（Homebrew 输出 3.x.y，官方二进制输出 v3.x.y）则直接用；否则与 ruff 同样经 uvx/pipx 取 shfmt-py。
  local have
  have=$(command shfmt --version 2>/dev/null || true)
  if [ -n "$have" ] && [ "${have#v}" = "$SHFMT_VERSION" ]; then
    command shfmt "$@"
  elif command -v uvx >/dev/null 2>&1; then
    uvx -q --from "shfmt-py==$SHFMT_PY_VERSION" shfmt "$@"
  else
    pipx run --spec "shfmt-py==$SHFMT_PY_VERSION" shfmt "$@"
  fi
}

shellcheck() {
  # 取法同 shfmt：本机已装同版本则直接用；否则经 uvx/pipx 取 shellcheck-py。
  local have
  have=$(command shellcheck --version 2>/dev/null | awk '$1 == "version:" { print $2 }' || true)
  if [ "$have" = "$SHELLCHECK_VERSION" ]; then
    command shellcheck "$@"
  elif command -v uvx >/dev/null 2>&1; then
    uvx -q --from "shellcheck-py==$SHELLCHECK_PY_VERSION" shellcheck "$@"
  else
    pipx run --spec "shellcheck-py==$SHELLCHECK_PY_VERSION" shellcheck "$@"
  fi
}

lizard() {
  if command -v uvx >/dev/null 2>&1; then
    uvx -q "lizard==$LIZARD_VERSION" "$@"
  else
    pipx run --spec "lizard==$LIZARD_VERSION" lizard "$@"
  fi
}

max_lines() {
  # 列出行数超过 MAX_LINES 的文件并非零退出。末尾加 /dev/null 保证 wc 总会多打一行合计，sed 把这一行去掉。
  wc -l -- "$@" /dev/null | sed '$d' | awk -v max="$MAX_LINES" '$1 > max { print; bad = 1 } END { exit bad }'
}

mapfile -d '' PY < <(files '*.py' '*.pyi')
# 进程替换的退出码不会自动传出：每个 < <(...) 之后 wait "$!" 取回，git ls-files 失败时随 set -e 退出。
wait "$!"
mapfile -d '' PRETTIER < <(files '*.js' '*.mjs' '*.cjs' '*.jsx' '*.ts' '*.mts' '*.cts' '*.tsx' \
  '*.md' '*.yml' '*.yaml' '*.json')
wait "$!"
mapfile -d '' SH < <(files '*.sh' '*.bash')
wait "$!"

if [ "$LINT" -eq 1 ]; then
  # 每一步直接调用工具并原样透出它的输出，非零即随 set -e 退出。
  mapfile -d '' CODE < <(files '*.py' '*.pyi' '*.sh' '*.bash' '*.cs' '*.ps1' '*.psm1' \
    '*.js' '*.mjs' '*.cjs' '*.jsx' '*.ts' '*.mts' '*.cts' '*.tsx')
  wait "$!"
  if [ ${#PY[@]} -gt 0 ]; then
    echo "== ruff check"
    ruff check --output-format concise -- "${PY[@]}"
    # 复杂度不认行内的 noqa，要放宽只能改这里。
    echo "== Python 函数圈复杂度不超过 $MAX_COMPLEXITY"
    ruff check --select C901 --ignore-noqa --config "lint.mccabe.max-complexity=$MAX_COMPLEXITY" \
      --output-format concise -- "${PY[@]}"
  fi
  if [ ${#SH[@]} -gt 0 ]; then
    echo "== shellcheck"
    shellcheck -- "${SH[@]}"
  fi
  mapfile -d '' CS < <(files '*.cs')
  wait "$!"
  if [ ${#CS[@]} -gt 0 ]; then
    echo "== C# 函数圈复杂度不超过 $MAX_COMPLEXITY"
    lizard -l csharp -C "$MAX_COMPLEXITY" -w -- "${CS[@]}"
    # 复杂度不认豁免：lizard 会放过带 #lizard forgives 注释的函数、注释里含 GENERATED CODE 的整个文件
    # 和仓根 whitelizard.txt 里列的函数，所以另查一遍。
    if grep -n -i -e 'lizard forgive' -e 'generated code' -- "${CS[@]}" || [ -e whitelizard.txt ]; then
      echo "lint: 不接受 lizard 的豁免注释（#lizard forgives、GENERATED CODE）与 whitelizard.txt" >&2
      exit 1
    fi
  fi
  echo "== 代码文件不超过 $MAX_LINES 行"
  max_lines "${CODE[@]}"
  echo "lint: ok"
  exit 0
fi

FAIL=0
if [ ${#PY[@]} -gt 0 ]; then
  if [ "$CHECK" -eq 1 ]; then
    ruff format --check --output-format concise -- "${PY[@]}" || FAIL=1
  else
    ruff format -- "${PY[@]}"
  fi
fi
if [ ${#PRETTIER[@]} -gt 0 ]; then
  if [ "$CHECK" -eq 1 ]; then
    npm_config_loglevel=warn npx --yes "prettier@$PRETTIER_VERSION" --check -- "${PRETTIER[@]}" || FAIL=1
  else
    npm_config_loglevel=warn npx --yes "prettier@$PRETTIER_VERSION" --write --log-level warn -- "${PRETTIER[@]}"
  fi
fi
if [ ${#SH[@]} -gt 0 ]; then
  if [ "$CHECK" -eq 1 ]; then
    # -l 在有不一致文件或解析失败时都非零退出，两种都算失败。
    rc=0
    bad=$(shfmt -l -- "${SH[@]}") || rc=$?
    if [ -n "$bad" ] || [ "$rc" -ne 0 ]; then
      printf 'shfmt: 需要重排：\n%s\n' "$bad"
      FAIL=1
    fi
  else
    shfmt -w -- "${SH[@]}"
  fi
fi

if [ "$CHECK" -eq 1 ]; then
  if [ "$FAIL" -eq 0 ]; then
    echo "format: ok"
  else
    echo "format: 格式不一致，运行 scripts/format.sh 修复" >&2
  fi
fi
exit "$FAIL"
