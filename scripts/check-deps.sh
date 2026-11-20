#!/usr/bin/env bash
# 依赖方向护栏：断言 REFACTOR_DESIGN.md §2.3 的全部禁止项（✗）。
# 用法：bash scripts/check-deps.sh（在仓库根目录运行，退出 0 为通过）。
# 只检查内部 import（blades-stock/...），不检查第三方依赖。
set -euo pipefail

cd "$(dirname "$0")/.."

GO_BIN="${GO_BIN:-$HOME/.local/go/bin/go}"
export GOTMPDIR="${GOTMPDIR:-$HOME/.cache/go-tmp}"
export TMPDIR="${TMPDIR:-$HOME/.cache/go-tmp}"

IMPORTS="$("$GO_BIN" list -f '{{.ImportPath}} {{join .Imports " "}}' ./... 2>/dev/null | grep -E '^blades-stock(\s|$)')"

FAIL=0

# assert_no_import <importer 包路径（精确匹配，空表示任意）> <被 import 的包> <规则说明>
assert_no_import() {
  local importer="$1" forbidden="$2" rule="$3"
  local hits
  if [ -z "$importer" ]; then
    hits=$(echo "$IMPORTS" | awk -v f="$forbidden" '{for (i=2;i<=NF;i++) if ($i==f) print $1}')
  else
    hits=$(echo "$IMPORTS" | awk -v p="$importer" -v f="$forbidden" '$1==p {for (i=2;i<=NF;i++) if ($i==f) print $1}')
  fi
  if [ -n "$hits" ]; then
    echo "FAIL: $rule"
    echo "$hits" | sed 's/^/  /'
    FAIL=1
  fi
}

# assert_no_import_prefix <importer 前缀> <被 import 的包前缀> <规则说明>
assert_no_import_prefix() {
  local prefix="$1" forbidden="$2" rule="$3"
  local hits
  hits=$(echo "$IMPORTS" | awk -v p="$prefix" -v f="$forbidden" 'index($1, p)==1 {for (i=2;i<=NF;i++) if (index($i, f)==1) print $1" -> "$i}')
  if [ -n "$hits" ]; then
    echo "FAIL: $rule"
    echo "$hits" | sed 's/^/  /'
    FAIL=1
  fi
}

MOD="blades-stock"
B="blades-stock/backend"

# --- 规则 2：基础包不依赖业务包 ---
# 基础包：models / util / paths / circuitbreaker / machineid / agent/types / logger
# （logger→paths 是唯一例外，现状保留）
for base in "$B/models" "$B/util" "$B/paths" "$B/circuitbreaker" "$B/machineid" "$B/agent/types" "$B/logger"; do
  for biz in "$B/data" "$B/datasource" "$B/quant" "$B/agent" "$B/agent/tools" "$B/agent/evaluator" "$B/services" "$B/strategy"; do
    assert_no_import "$base" "$biz" "规则2：基础包 $base 不得 import 业务包 $biz"
  done
done
# logger 不得 import models/util/circuitbreaker/machineid/types/db（只允许 paths）
for other in "$B/models" "$B/util" "$B/circuitbreaker" "$B/machineid" "$B/agent/types" "$B/db"; do
  assert_no_import "$B/logger" "$other" "规则2：logger 只允许 import paths，不得 import $other"
done
# models/util/paths/circuitbreaker/machineid/types 之间互不 import
for base in "$B/models" "$B/util" "$B/paths" "$B/circuitbreaker" "$B/machineid" "$B/agent/types"; do
  for other in "$B/models" "$B/util" "$B/paths" "$B/circuitbreaker" "$B/machineid" "$B/agent/types" "$B/db" "$B/logger"; do
    [ "$base" = "$other" ] && continue
    assert_no_import "$base" "$other" "规则2：基础包 $base 不得 import $other"
  done
done

# --- db：只允许 logger/models/paths ---
for forbidden in "$B/data" "$B/datasource" "$B/quant" "$B/agent" "$B/agent/tools" "$B/agent/evaluator" "$B/services" "$B/strategy" "$B/util" "$B/circuitbreaker" "$B/machineid" "$B/agent/types"; do
  assert_no_import "$B/db" "$forbidden" "规则：db 不得 import $forbidden"
done

# --- datasource 是叶子：只依赖基础包；只有 data 能 import 它 ---
# 基础包（models/util/paths/circuitbreaker/machineid/types/logger）允许 import，
# db 不允许；业务包一律不允许。
assert_no_import "$B/datasource" "$B/db" "规则3：datasource 不得 import db"
for biz in "$B/quant" "$B/agent" "$B/agent/tools" "$B/agent/evaluator" "$B/services" "$B/strategy" "$B/data"; do
  assert_no_import "$B/datasource" "$biz" "规则3：datasource 不得 import 业务包 $biz"
done
# 除 data 外，任何包不得 import datasource（R-23 规则）
while IFS= read -r line; do
  pkg="${line%% *}"
  rest="${line#* }"
  case " $rest " in
    *" $B/datasource "*) ;;
    *) continue ;;
  esac
  if [ "$pkg" != "$B/data" ]; then
    echo "FAIL: 规则3：只有 backend/data 能 import backend/datasource，$pkg 违规"
    FAIL=1
  fi
done <<< "$IMPORTS"

# --- strategy：零内部依赖 ---
while IFS= read -r line; do
  pkg="${line%% *}"
  rest="${line#* }"
  if [ "$pkg" = "$B/strategy" ]; then
    for imp in $rest; do
      case "$imp" in
        "$MOD"/*)
          echo "FAIL: 规则：strategy 必须零内部依赖，实际 import $imp"
          FAIL=1
          ;;
      esac
    done
  fi
done <<< "$IMPORTS"

# --- data：不得 import agent / tools / services / quant ---
for forbidden in "$B/agent" "$B/agent/tools" "$B/agent/evaluator" "$B/services" "$B/quant"; do
  assert_no_import "$B/data" "$forbidden" "规则：data 不得 import $forbidden"
done

# --- quant：不得 import agent / tools / services / datasource ---
for forbidden in "$B/agent" "$B/agent/tools" "$B/agent/evaluator" "$B/services" "$B/datasource"; do
  assert_no_import "$B/quant" "$forbidden" "规则：quant 不得 import $forbidden"
done

# --- agent/tools：不得 import agent / services / evaluator / datasource ---
for forbidden in "$B/agent" "$B/services" "$B/agent/evaluator" "$B/datasource"; do
  assert_no_import "$B/agent/tools" "$forbidden" "规则：agent/tools 不得 import $forbidden"
done
# agent/tools/data 子包：不得 import backend/agent 本身（精确匹配；父包 tools 允许）、
# services / evaluator / datasource
assert_no_import "$B/agent/tools/data" "$B/agent" "规则：agent/tools/data 不得 import backend/agent"
for forbidden in "$B/services" "$B/agent/evaluator" "$B/datasource"; do
  assert_no_import "$B/agent/tools/data" "$forbidden" "规则：agent/tools/data 不得 import $forbidden"
done

# --- agent：不得 import services / evaluator / datasource ---
for forbidden in "$B/services" "$B/agent/evaluator" "$B/datasource"; do
  assert_no_import "$B/agent" "$forbidden" "规则4：agent 不得 import $forbidden"
done

# --- evaluator：不得 import services / data / quant / tools / datasource / strategy ---
for forbidden in "$B/services" "$B/data" "$B/datasource" "$B/quant" "$B/agent/tools" "$B/strategy"; do
  assert_no_import "$B/agent/evaluator" "$forbidden" "规则4：evaluator 不得 import $forbidden"
done

# --- services：不得 import datasource ---
assert_no_import "$B/services" "$B/datasource" "规则：services 不得 import datasource"

# --- 交付层：根包 / cmd 不得 import datasource；backend 永不 import 根包/cmd ---
for pkg in "$MOD" "$MOD/cmd/server"; do
  assert_no_import "$pkg" "$B/datasource" "规则5：交付层 $pkg 不得 import datasource"
done
assert_no_import_prefix "$MOD/cmd/" "$B/datasource" "规则5：cmd/* 不得 import datasource"
# backend 包不得 import 根包（根包即模块本身）或 cmd
while IFS= read -r line; do
  pkg="${line%% *}"
  rest="${line#* }"
  case "$pkg" in
    "$B"/*)
      for imp in $rest; do
        if [ "$imp" = "$MOD" ] || [ "$imp" = "$MOD/cmd/server" ] || [[ "$imp" == "$MOD/cmd/"* ]]; then
          echo "FAIL: 规则5：backend 包 $pkg 不得 import 交付层 $imp"
          FAIL=1
        fi
      done
      ;;
  esac
done <<< "$IMPORTS"

if [ "$FAIL" -eq 0 ]; then
  echo "check-deps: OK（依赖方向规则全部通过）"
  exit 0
else
  echo "check-deps: FAILED（见上）"
  exit 1
fi
