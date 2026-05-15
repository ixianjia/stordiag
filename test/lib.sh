#!/usr/bin/env bash
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STORDIAG="${STORDIAG:-$LIB_DIR/../stordiag}"
export STORDIAG

say()  { printf "\033[1;34m>>> %s\033[0m\n" "$*"; }
ok()   { printf "\033[1;32m[✓]\033[0m %s\n" "$*"; }
fail() { printf "\033[1;31m[✗]\033[0m %s\n" "$*" >&2; return 1; }
warn() { printf "\033[1;33m[!]\033[0m %s\n" "$*"; }

STORDIAG_TIMEOUT=30
export STORDIAG_TIMEOUT

CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
DIAG_DIR="/tmp/stordiag_test_$$"
mkdir -p "$DIAG_DIR"

check_deps() {
  local missing=0
  command -v "$STORDIAG" >/dev/null 2>&1 || { warn "stordiag 未找到，先构建"; make -C "$(dirname "$STORDIAG")" build; }
  command -v jq >/dev/null 2>&1 || { warn "jq 未安装 (apt/yum install jq)"; }
  command -v "$CONTAINER_RUNTIME" >/dev/null 2>&1 && ok "container runtime: $CONTAINER_RUNTIME" || warn "$CONTAINER_RUNTIME 未安装，跳过需要容器的场景"
}

minio_start() {
  local name="$1" port="$2"
  shift 2

  $CONTAINER_RUNTIME rm -f "$name" 2>/dev/null || true
  $CONTAINER_RUNTIME run -d --name "$name" --privileged \
    -p "$port":9000 \
    -e MINIO_ROOT_USER=minioadmin \
    -e MINIO_ROOT_PASSWORD=minioadmin \
    "$@" \
    quay.io/minio/minio server /data --console-address ":9001" >/dev/null
  sleep 3
  ok "MinIO '$name' 已启动 (port $port)"
}

minio_stop() {
  local name="$1"
  $CONTAINER_RUNTIME rm -f "$name" 2>/dev/null || true
  ok "MinIO '$name' 已停止"
}

stordiag_doctor() {
  local label="$1" out="$2"
  shift 2
  local json_out="$DIAG_DIR/${label}.json"
  say "诊断: $label"
  # 内联 export 确保环境变量传递到子进程
  export STORDIAG_ENDPOINT="${STORDIAG_ENDPOINT:-localhost:9000}"
  export STORDIAG_ACCESS_KEY="${STORDIAG_ACCESS_KEY:-minioadmin}"
  export STORDIAG_SECRET_KEY="${STORDIAG_SECRET_KEY:-minioadmin}"
  export STORDIAG_BUCKET="${STORDIAG_BUCKET:-stordiag}"
  export STORDIAG_TIMEOUT="${STORDIAG_TIMEOUT:-30}"
  "$STORDIAG" doctor --json "$@" 2>/dev/null > "$json_out" || true
  cp "$json_out" "$out"
  ok "诊断结果: $out"
}

analyse() {
  local json="$1"
  printf "\n"
  say "===== 分层分析 ====="
  jq -r '.Layers[] | "  \(.label) [\(.layer)]:   OK=\(.ok)  WARN=\(.warn)  FAIL=\(.fail)"' "$json" 2>/dev/null

  local failed
  failed=$(jq -r '.Layers[].probes[] | select(.status=="FAIL") | "  [FAIL] \(.name): \(.value)"' "$json" 2>/dev/null)
  if [ -n "$failed" ]; then
    printf "\n  \033[1;31m!!! FAIL 探针:\033[0m\n%s\n" "$failed"
  fi

  local warned
  warned=$(jq -r '.Layers[].probes[] | select(.status=="WARN") | "  [WARN] \(.name): \(.value)"' "$json" 2>/dev/null)
  if [ -n "$warned" ]; then
    printf "\n  \033[1;33m!!! WARN 探针:\033[0m\n%s\n" "$warned"
  fi

  local summary
  summary=$(jq -r '.Summary' "$json" 2>/dev/null)
  case "$summary" in
    FAIL*) printf "\n  \033[1;31m结论: %s\033[0m\n" "$summary" ;;
    PASS*) printf "\n  \033[1;32m结论: %s\033[0m\n" "$summary" ;;
    *)     printf "\n  结论: %s\n" "$summary" ;;
  esac
}

cleanup() {
  rm -rf "$DIAG_DIR"
}

trap cleanup EXIT INT TERM
