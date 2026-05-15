#!/usr/bin/env bash
# 场景 4: 服务完全不可用
# 模拟: MinIO 进程被 kill
# 预期: L1 ping FAIL, L2 TCP connect 正常或 FAIL

source "$(dirname "$0")/../lib.sh"

SCENARIO="04-service-down"
OUT_DIR="${1:-/tmp/stordiag_scenarios}"
mkdir -p "$OUT_DIR"

CONTAINER_NAME="stordiag-s4-minio"
MINIO_PORT=9004

if ! command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1; then
  warn "$CONTAINER_RUNTIME 不可用，跳过服务中断场景"
  exit 0
fi

say "场景 $SCENARIO: 服务完全不可用"

say "步骤 1/3: 启动 MinIO"
minio_start "$CONTAINER_NAME" "$MINIO_PORT"

say "步骤 2/3: 获取基线诊断 (服务正常)"
BASELINE="$OUT_DIR/${SCENARIO}_baseline.json"
export STORDIAG_ENDPOINT="localhost:$MINIO_PORT"
export STORDIAG_ACCESS_KEY=minioadmin
export STORDIAG_SECRET_KEY=minioadmin
export STORDIAG_BUCKET=stordiag
export STORDIAG_TIMEOUT=10
stordiag_doctor "${SCENARIO}_baseline" "$BASELINE" --layers all
analyse "$BASELINE"

say "步骤 3/3: 杀死 MinIO 进程后诊断"
$CONTAINER_RUNTIME exec "$CONTAINER_NAME" kill -KILL 1 2>/dev/null || true
sleep 2

DOWN="$OUT_DIR/${SCENARIO}_down.json"
export STORDIAG_ENDPOINT="localhost:$MINIO_PORT"
export STORDIAG_ACCESS_KEY=minioadmin
export STORDIAG_SECRET_KEY=minioadmin
export STORDIAG_BUCKET=stordiag
export STORDIAG_TIMEOUT=5
stordiag_doctor "${SCENARIO}_down" "$DOWN" --layers all
analyse "$DOWN"

minio_stop "$CONTAINER_NAME"

say "===== 结论 ====="
printf "\n  \033[1;36m关键观察:\033[0m\n"
printf "  - ping FAIL — 应用层不可达\n"
printf "  - bench FAIL / data_integrity FAIL\n"
printf "  - tcp_connect 可能正常 (端口在监听但无响应) 或 FAIL\n"
printf "  - 系统层应正常 (这是应用故障, 非系统故障)\n"
printf "  \033[1;33m根因定位: L1+L2 — 服务进程异常\033[0m\n"
