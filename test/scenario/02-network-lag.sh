#!/usr/bin/env bash
# 场景 2: 网络高延迟 + 丢包
# 模拟: podman + tc netem
# 需要: podman + --privileged 容器

source "$(dirname "$0")/../lib.sh"

SCENARIO="02-network-lag"
OUT_DIR="${1:-/tmp/stordiag_scenarios}"
mkdir -p "$OUT_DIR"

CONTAINER_NAME="stordiag-s2-minio"
MINIO_PORT=9002

if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
  warn "$CONTAINER_RUNTIME 不可用，跳过网络延迟场景"
  exit 0
fi

say "场景 $SCENARIO: 网络高延迟 + 丢包"

say "步骤 1/4: 启动 MinIO"
minio_start "$CONTAINER_NAME" "$MINIO_PORT"

say "步骤 2/4: 获取基线诊断 (无延迟)"
BASELINE="$OUT_DIR/${SCENARIO}_baseline.json"
export STORDIAG_ENDPOINT="localhost:$MINIO_PORT"
export STORDIAG_ACCESS_KEY=minioadmin
export STORDIAG_SECRET_KEY=minioadmin
export STORDIAG_BUCKET=stordiag
stordiag_doctor "${SCENARIO}_baseline" "$BASELINE" --layers all
analyse "$BASELINE"

say "步骤 3/4: 注入网络延迟 + 丢包"
$CONTAINER_RUNTIME exec "$CONTAINER_NAME" tc qdisc replace dev eth0 root netem delay 100ms loss 5% 2>&1 || \
  warn "tc 注入失败，可能需要 --cap-add=NET_ADMIN"
ok "注入完成: delay 100ms + loss 5%"

say "步骤 4/4: 延迟下诊断"
LAG="$OUT_DIR/${SCENARIO}_lag.json"
export STORDIAG_ENDPOINT="localhost:$MINIO_PORT"
export STORDIAG_ACCESS_KEY=minioadmin
export STORDIAG_SECRET_KEY=minioadmin
export STORDIAG_BUCKET=stordiag
export STORDIAG_TIMEOUT=15
stordiag_doctor "${SCENARIO}_lag" "$LAG" --layers all
analyse "$LAG"

say "清理: 移除网络延迟"
$CONTAINER_RUNTIME exec "$CONTAINER_NAME" tc qdisc del dev eth0 root 2>/dev/null || true
minio_stop "$CONTAINER_NAME"

say "===== 结论 ====="
printf "\n  \033[1;36m关键观察:\033[0m\n"
printf "  - tcp_connect 延迟应从 <1ms 升至 ~100ms\n"
printf "  - bench 吞吐应显著下降\n"
printf "  - 系统层应正常 (磁盘 IO 无变化)\n"
printf "  \033[1;33m根因定位: L2 — 网络层延迟/丢包\033[0m\n"
