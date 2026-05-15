#!/usr/bin/env bash
# stordiag 生产场景复现测试套件
# 一键执行所有场景并生成分析报告
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
source "$ROOT/lib.sh"

OUT_DIR="${1:-/tmp/stordiag_scenarios}"
SUMMARY="$OUT_DIR/summary.json"
mkdir -p "$OUT_DIR"

BOLD="\033[1m"
GREEN="\033[1;32m"
YELLOW="\033[1;33m"
RED="\033[1;31m"
CYAN="\033[1;36m"
RESET="\033[0m"

export STORDIAG="${STORDIAG:-$ROOT/../stordiag}"

banner() {
  clear 2>/dev/null || true
  printf "${CYAN}"
  cat <<'BANNER'
╔══════════════════════════════════════════════════╗
║         stordiag 生产场景复现测试套件             ║
║  分层诊断工具 x 4 种真实故障场景                  ║
╚══════════════════════════════════════════════════╝
BANNER
  printf "${RESET}\n"
}

print_header() {
  local n="$1" name="$2" desc="$3"
  printf "${BOLD}${CYAN}"
  printf "┌─────────────────────────────────────────┐\n"
  printf "│ 场景 %s: %-33s │\n" "$n" "$name"
  printf "│ %-41s │\n" "$desc"
  printf "└─────────────────────────────────────────┘${RESET}\n\n"
}

run_scenario() {
  local n="$1" script="$2"
  local json="$OUT_DIR/${n}.json"
  local meta="$OUT_DIR/${n}_meta.json"

  printf "${BOLD}执行场景 %s...${RESET}\n\n" "$n"
  if ! bash "$script" "$OUT_DIR"; then
    warn "场景 $n 未完全执行 (可能缺失依赖)"
    return
  fi

  # 找最新的诊断 JSON
  local latest
  latest=$(ls -t "$OUT_DIR"/"${n}"_*.json 2>/dev/null | head -1 || true)
  if [ -n "$latest" ] && [ "$latest" != "$meta" ]; then
    cp "$latest" "$json"

    # 提取关键指标
    jq --arg scenario "$n" '{
      scenario: $scenario,
      summary: .Summary,
      layers: [.Layers[] | {layer: .layer, label: .label, ok: .ok, warn: .warn, fail: .fail}],
      failures: [.Layers[].probes[] | select(.status=="FAIL") | {name: .name, value: .value}],
      warnings: [.Layers[].probes[] | select(.status=="WARN") | {name: .name, value: .value}]
    }' "$json" > "$meta" 2>/dev/null || true
  fi

  printf "\n"
}

print_summary() {
  local total=0 passed=0 failed=0

  printf "${BOLD}${CYAN}═════════════════════════════════════════${RESET}\n"
  printf "${BOLD}${CYAN}          最终汇总报告                   ${RESET}\n"
  printf "${BOLD}${CYAN}═════════════════════════════════════════${RESET}\n\n"

  for meta in "$OUT_DIR"/*_meta.json; do
    [ -f "$meta" ] || continue
    local scenario
    scenario=$(jq -r '.scenario' "$meta")
    local summary
    summary=$(jq -r '.summary' "$meta")
    local layers
    layers=$(jq -r '.layers[] | "    \(.label) [\(.layer)]:  OK=\(.ok)  WARN=\(.warn)  FAIL=\(.fail)"' "$meta" 2>/dev/null)
    local failures
    failures=$(jq -r '.failures[] | "    \(.name): \(.value)"' "$meta" 2>/dev/null)
    local warnings
    warnings=$(jq -r '.warnings[] | "    \(.name): \(.value)"' "$meta" 2>/dev/null)

    total=$((total + 1))
    case "$summary" in
      FAIL*) passed=$((passed + 1)) ;;&  # 故障场景预期 FAIL
      PASS*) passed=$((passed + 1)) ;;&
      *) true ;;
    esac

    printf "${BOLD}场景 %s${RESET}\n" "$scenario"
    printf "%s\n" "$layers"
    if [ -n "$failures" ]; then
      printf "  ${RED}FAILURES:${RESET}\n%s\n" "$failures"
    fi
    if [ -n "$warnings" ]; then
      printf "  ${YELLOW}WARNINGS:${RESET}\n%s\n" "$warnings"
    fi

    # 根因建议
    local suggestion
    case "$scenario" in
      *01*)  suggestion="L3 磁盘 IO 饱和 → 检查磁盘健康 / iostat -x 1 / bpftrace IO 追踪" ;;
      *02*)  suggestion="L2 网络延迟/丢包 → 检查网络 / mtr / tcpdump / 联系网络团队" ;;
      *03*)  suggestion="L1 数据完整性 → 检查存储后端校验机制 / 启用端到端 checksum" ;;
      *04*)  suggestion="L1+L2 服务不可用 → 检查应用日志 / 重启服务 / 配置 HA" ;;
      *)     suggestion="—" ;;
    esac
    printf "  ${CYAN}建议:${RESET} %s\n" "$suggestion"
    printf "\n"
  done

  printf "${BOLD}${CYAN}═════════════════════════════════════════${RESET}\n"
  printf "共执行 ${BOLD}%d${RESET} 个场景\n" "$total"
  printf "${CYAN}诊断报告 JSON 已保存至:${RESET} %s/{01..04}.json\n" "$OUT_DIR"
  printf "${BOLD}${YELLOW}提示:${RESET} 故障场景预期 FAIL, 正常场景预期 PASS\n\n"
}

# ====== Main ======
banner
check_deps

run_scenario "01-disk-io"       "$ROOT/scenario/01-disk-io.sh"
run_scenario "02-network-lag"   "$ROOT/scenario/02-network-lag.sh"
run_scenario "03-data-corrupt"  "$ROOT/scenario/03-data-corrupt.sh"
run_scenario "04-service-down"  "$ROOT/scenario/04-service-down.sh"

print_summary

printf "${BOLD}${GREEN}分析完成!${RESET}\n"
printf "结果目录: ${CYAN}%s${RESET}\n" "$OUT_DIR"
printf "快速查看:\n"
printf "  cat %s/01-disk-io.json          | jq '.Summary'\n" "$OUT_DIR"
printf "  cat %s/02-network-lag.json       | jq '.Layers[1].probes[]'\n" "$OUT_DIR"
printf "  cat %s/03-data-corrupt.json      | jq '.Layers[0].probes[]'\n" "$OUT_DIR"
printf "  cat %s/04-service-down.json      | jq '.Layers'\n" "$OUT_DIR"
