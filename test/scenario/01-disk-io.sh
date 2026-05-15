#!/usr/bin/env bash
# 场景 1: 磁盘 IO 打满
# 模拟: stress-ng 制造高 IO 压力
# 预期: L3 系统层 await↑ pressure_io↑

source "$(dirname "$0")/../lib.sh"

SCENARIO="01-disk-io"
OUT_DIR="${1:-/tmp/stordiag_scenarios}"
mkdir -p "$OUT_DIR"

say "场景 $SCENARIO: 磁盘 IO 打满"

mkdir -p /tmp/stordiag_s1

say "步骤 1/3: 获取基线诊断"

BASELINE="$OUT_DIR/${SCENARIO}_baseline.json"
export STORDIAG_ENDPOINT=/tmp/stordiag_s1
stordiag_doctor "${SCENARIO}_baseline" "$BASELINE" --layers all
analyse "$BASELINE"

say "步骤 2/3: 启动 IO 压力 (stress-ng --hdd 4 --timeout 60s)"
if ! command -v stress-ng >/dev/null 2>&1; then
  warn "stress-ng 未安装，尝试用 dd 模拟"
  for i in $(seq 1 4); do
    dd if=/dev/zero of=/tmp/stordiag_s1_stress_$i bs=1M count=1024 conv=fsync 2>/dev/null &
  done
else
  stress-ng --hdd 4 --hdd-bytes 4G --timeout 60s &>/dev/null &
fi

say "等待 5 秒让 IO 压力稳定..."
sleep 5

say "步骤 3/3: 压力下诊断"
STRESS="$OUT_DIR/${SCENARIO}_stress.json"
export STORDIAG_ENDPOINT=/tmp/stordiag_s1
stordiag_doctor "${SCENARIO}_stress" "$STRESS" --layers all
analyse "$STRESS"

wait 2>/dev/null || true
kill %1 2>/dev/null || true
rm -f /tmp/stordiag_s1_stress_*

say "===== 结论 ====="
BASELINE_BENCH=$(jq -r '.Layers[0].probes[] | select(.name=="bench_write") | .value // "N/A"' "$BASELINE" 2>/dev/null)
STRESS_BENCH=$(jq -r '.Layers[0].probes[] | select(.name=="bench_write") | .value // "N/A"' "$STRESS" 2>/dev/null)
BASELINE_DISK=$(jq -r '.Layers[2].probes[] | select(.name | startswith("disk_")) | .value // "N/A"' "$BASELINE" 2>/dev/null)
STRESS_DISK=$(jq -r '.Layers[2].probes[] | select(.name | startswith("disk_")) | .value // "N/A"' "$STRESS" 2>/dev/null)
printf "  基线吞吐: %s\n  压力吞吐: %s\n  基线 IO:   %s\n  压力 IO:   %s\n" \
  "$BASELINE_BENCH" "$STRESS_BENCH" "$BASELINE_DISK" "$STRESS_DISK"

printf "\n  \033[1;36m关键观察:\033[0m\n"
printf "  - IO 压力下 bench 吞吐应下降, await 应上升\n"
printf "  - pressure_io avg60 应 > 0\n"
printf "  - 网络层应正常 (N/A 因为本地)\n"
printf "  \033[1;33m根因定位: L3 — 系统层磁盘 IO 饱和\033[0m\n"
