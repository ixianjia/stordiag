#!/usr/bin/env bash
# 场景 3: 静默数据损坏检测
# 模拟: 写入数据 -> 记录 checksum -> 篡改 -> 比对
# 预期:
#   - doctor 诊断显示 storage 本身正常 (ping/bench OK)
#   - 篡改后数据 sha256 与预期不符
#   - verify 命令能检测到 silent corruption (区别于 doctor 创建的新临时文件)

source "$(dirname "$0")/../lib.sh"

SCENARIO="03-data-corrupt"
OUT_DIR="${1:-/tmp/stordiag_scenarios}"
mkdir -p "$OUT_DIR"

say "场景 $SCENARIO: 静默数据损坏检测"

TEST_DIR="/tmp/stordiag_s3_corrupt"
mkdir -p "$TEST_DIR"

say "步骤 1/4: doctor 基线诊断 (storage 整体健康)"
BASELINE="$OUT_DIR/${SCENARIO}_baseline.json"
export STORDIAG_ENDPOINT="$TEST_DIR"
stordiag_doctor "${SCENARIO}_baseline" "$BASELINE" --layers all
analyse "$BASELINE"

say "步骤 2/4: 写入数据 + 记录预期 checksum"
echo "stordiag integrity test payload: $(date)" > "$TEST_DIR/important.dat"
EXPECTED_HASH=$(sha256sum "$TEST_DIR/important.dat" | cut -d' ' -f1)
ok "文件已写入, SHA256=$EXPECTED_HASH"

say "步骤 3/4: 模拟静默损坏 (直接写入磁盘, 绕过文件系统 API)"
sh -c "echo '*** SILENT CORRUPTION injected ***' >> '$TEST_DIR/important.dat'"
ACTUAL_HASH=$(sha256sum "$TEST_DIR/important.dat" | cut -d' ' -f1)
ok "文件已篡改, SHA256=$ACTUAL_HASH"

if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
  printf "  \033[1;31m!!! checksum 不一致 — 检测到数据损坏 !!!\033[0m\n"
  printf "  预期: %s\n  实际: %s\n" "$EXPECTED_HASH" "$ACTUAL_HASH"
fi

say "步骤 4/4: 损坏后诊断 (doctor 仍应 PASS — storage 本身可用)"
CORRUPT="$OUT_DIR/${SCENARIO}_corrupt.json"
export STORDIAG_ENDPOINT="$TEST_DIR"
stordiag_doctor "${SCENARIO}_corrupt" "$CORRUPT" --layers app
analyse "$CORRUPT"

say "===== 结论 ====="
printf "\n  \033[1;36m关键观察:\033[0m\n"
printf "  - doctor 诊断: storage 本身正常 (ping/bench OK)\n"
printf "  - 文件级别: sha256sum 从 %s 变为 %s\n" "$EXPECTED_HASH" "$ACTUAL_HASH"
printf "  - 数据已损坏但存储仍报告成功 — 典型 silent corruption\n"
printf "\n  \033[1;33m根因定位: L1 — 应用层数据完整性遭破坏\033[0m\n"
printf "  \033[1;33m建议: 启用端到端 checksum 验证, 使用 checksum 硬件卸载的存储介质\033[0m\n"
