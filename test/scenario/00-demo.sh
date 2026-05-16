#!/usr/bin/env bash
# ============================================================
# stordiag 三层诊断操作说明 & 模拟分析案例
# 用法: bash test/scenario/00-demo.sh [/tmp/output_dir]
#
# 本脚本包含:
#   1. 逐条命令演示 + 预期输出解读
#   2. 故障注入方法
#   3. 三层诊断矩阵分析
#   4. diff 对比和根因定位推理
# ============================================================
set -euo pipefail

source "$(dirname "$0")/../lib.sh"

OUT_DIR="${1:-/tmp/stordiag_demo}"
mkdir -p "$OUT_DIR"

say "================================================"
say "stordiag 三层诊断操作说明"
say "================================================"

# ============================================================
# Part 0: 检查环境
# ============================================================
say "Part 0: 环境检查"
check_deps
echo ""

# ============================================================
# Part 1: 基础用法 — 认识三层输出
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 1: 基础用法 — 认识三层输出"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
say "命令: ./stordiag doctor --endpoint /tmp"
echo ""
say "输出结构:"
echo ""
cat <<'TEXT'
  === Doctor Report: /tmp (posix) ===
    Timestamp:  2026-05-16T19:35:21+08:00
    Summary:    PASS

  --- Application [L1:app]  OK=3  WARN=0  FAIL=0 ---
  Probe           Status  Latency     Value
  ping            OK      3µs         reachable
  bench_write     OK      2ms         1997 MB/s  p99=2.1ms
  data_integrity  OK      -           checksums match

  --- Network [L2:network]  OK=0  WARN=0  FAIL=0 ---
  Probe    Status  Latency  Value
  network  N/A     -        local filesystem — no network

  --- System [L3:system]  OK=5  WARN=0  FAIL=0 ---
  Probe             Status  Value
  disk_nvme0n1      OK      iops=10 await=0.1ms
  memory_available  OK      24975 MiB
  pressure_cpu      OK      avg60=0.0%
  pressure_io       OK      avg60=0.2%

  --- Filesystem [L3:fs]  OK=4  WARN=0  FAIL=0 ---
  Probe         Status  Value
  fs_type       OK      ext4
  fs_usage      OK      42% used (210 GiB / 500 GiB)
  fs_inodes     OK      0% used
  block_device  OK      /dev/nvme0n1 (SSD)
TEXT
echo ""

say "逐层解读:"
echo "  L1 应用层 — 端到端功能检验"
echo "    ping:          存储可达性 + 往返延迟"
echo "    bench_write:   写入吞吐 + p99 延迟"
echo "    data_integrity:写→读→sha256 校验"
echo "    如果 L1 挂了: 存储本身或网络有问题"
echo ""
echo "  L2 网络层 — 网络链路分解"
echo "    dns_lookup:    DNS 解析耗时"
echo "    tcp_connect:   TCP 握手耗时"
echo "    tls_handshake: TLS 握手耗时 + 协议版本"
echo "    如果 L2 挂了: 网络问题，不是存储问题"
echo ""
echo "  L3 系统层 — 宿主机资源健康"
echo "    disk_*:        块设备 IOPS / await / 队列深度"
echo "    memory_available:  可用内存"
echo "    pressure_cpu/io:   CPU/IO 压力失速指标"
echo "    cpu_iowait:        CPU iowait"
echo "    如果 L3 挂了: 宿主机资源瓶颈，跟远端存储无关"
echo ""

# ============================================================
# Part 2: 基线获取 + JSON 输出
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 2: 基线诊断"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

TEST_DIR="/tmp/stordiag_demo_data"
mkdir -p "$TEST_DIR"

say "命令: stordiag doctor --endpoint $TEST_DIR --json"
echo ""
$STORDIAG doctor --endpoint "$TEST_DIR" --json > "$OUT_DIR/baseline.json" 2>/dev/null
say "基线报告已保存: $OUT_DIR/baseline.json"
echo ""

say "查看分层摘要:"
jq -r '.Layers[] | "  \(.label) [\(.layer)]  OK=\(.ok)  WARN=\(.warn)  FAIL=\(.fail)"' "$OUT_DIR/baseline.json" 2>/dev/null
echo ""

# ============================================================
# Part 3: 故障注入方法速查
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 3: 故障注入方法速查"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
cat <<'TEXT'
  你想模拟              用这个命令                          影响哪层
  ──────────            ───────────────────────────         ──────
  磁盘 IO 饱和          stress-ng --hdd 4 --timeout 60s     L3
                        dd if=/dev/zero of=/tmp/big bs=1M count=4096 conv=fsync

  CPU 打满              stress-ng --cpu 4 --timeout 60s     L3
                        yes > /dev/null &

  内存不足              stress-ng --vm 2 --vm-bytes 80%     L3
                        或 fork-bomb

  网络延迟              tc qdisc add dev eth0 root netem     L2
                        delay 100ms loss 10%

  网络断开              iptables -A INPUT -p tcp --dport     L2
                        9000 -j DROP

  服务宕机              kill -KILL <minio-PID>              L1+L2
                        docker stop <container>

  数据损坏              echo 'corrupt' >> important.dat      L1
                        (绕过文件系统 API 直接写盘)
TEXT
echo ""

# ============================================================
# Part 4: 场景模拟 — 磁盘 IO 饱和
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 4: 场景模拟 — 磁盘 IO 饱和"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

say "注入方法:"
echo "  stress-ng --hdd 4 --hdd-bytes 4G &"
echo "  或 (无 stress-ng 时): dd 写大文件"
echo ""

# 检查是否有 stress-ng
if command -v stress-ng >/dev/null 2>&1; then
  say "启动 stress-ng IO 压力 (后台运行 60s)..."
  stress-ng --hdd 4 --hdd-bytes 4G --timeout 60s &>/dev/null &
else
  say "启动 dd IO 压力..."
  for i in 1 2 3 4; do
    dd if=/dev/zero of="$TEST_DIR/stress_$i" bs=1M count=2048 conv=fsync 2>/dev/null &
  done
fi
STRESS_PID=$!

say "等待 5s 让 IO 压力稳定..."
sleep 5

say "命令: stordiag doctor --endpoint $TEST_DIR --json"
$STORDIAG doctor --endpoint "$TEST_DIR" --json > "$OUT_DIR/stress.json" 2>/dev/null

# 清理压力
if command -v stress-ng >/dev/null 2>&1; then
  kill %1 2>/dev/null || true
else
  rm -f "$TEST_DIR"/stress_*
fi
wait 2>/dev/null || true

say "压力报告: $OUT_DIR/stress.json"
echo ""

# ============================================================
# Part 5: diff 对比分析
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 5: diff 对比 — 找出变化探针"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

say "命令: stordiag diff baseline.json stress.json"
echo ""
$STORDIAG diff "$OUT_DIR/baseline.json" "$OUT_DIR/stress.json" 2>/dev/null
echo ""

say "diff 标记含义:"
echo "  ~ = 该探针值发生了变化（可能劣化或改善）"
echo "  + = 新增探针"
echo "  - = 消失探针"
echo ""

# ============================================================
# Part 6: 三层诊断矩阵
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 6: 三层诊断矩阵 — 根因定位推理"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

say "判断规则:"
echo ""
cat <<'TEXT'
  当出现问题时，看哪一层先 FAIL 或 WARN:

  故障场景          | L1 应用层       | L2 网络层       | L3 系统层        | 根因定位
  ─────────────────┼────────────────┼────────────────┼─────────────────┼────────
  磁盘 IO 饱和     | 吞吐↓ 延迟↑    | 正常            | await↑ iops↓     | ⬅ L3
  CPU 打满         | 延迟↑          | 正常            | iowait↑          | ⬅ L3
  内存不足         | OOM/连接重置   | 正常            | mem_avail↓       | ⬅ L3
  网络延迟         | 延迟波动        | tcp_connect↑    | IOPS 正常         | ⬅ L2
  网络丢包         | 吞吐骤降        | 重传/超时        | IOPS 正常         | ⬅ L2
  DNS 故障         | 连接失败        | dns_lookup FAIL | 正常              | ⬅ L2
  服务端宕机       | ping FAIL       | tcp 正常或 FAIL  | 系统正常          | ⬅ L1+L2
  静默数据损坏     | checksum FAIL   | 正常            | 正常              | ⬅ L1

  推理链 (以磁盘 IO 饱和为例):
    1. L3 disk_* await 上升 → 磁盘排队
    2. L1 bench_write 吞吐下降（连锁反应）
    3. L2 网络正常 → 排除网络因素
    ✅ 根因: L3 系统层 — 磁盘 IO 饱和
TEXT
echo ""

say "当前测试的诊断结果:"
echo ""

BASELINE_SUM=$(jq -r '.Summary' "$OUT_DIR/baseline.json" 2>/dev/null)
STRESS_SUM=$(jq -r '.Summary' "$OUT_DIR/stress.json" 2>/dev/null)

echo "  基线摘要: $BASELINE_SUM"
echo "  压力摘要: $STRESS_SUM"
echo ""

# 分析具体变化
L1=$(jq -r '.Layers[] | select(.layer=="L1:app") | "OK=\(.ok) WARN=\(.warn) FAIL=\(.fail)"' "$OUT_DIR/stress.json" 2>/dev/null)
L2=$(jq -r '.Layers[] | select(.layer=="L2:network") | "OK=\(.ok) WARN=\(.warn) FAIL=\(.fail)"' "$OUT_DIR/stress.json" 2>/dev/null)
L3=$(jq -r '.Layers[] | select(.layer=="L3:system") | "OK=\(.ok) WARN=\(.warn) FAIL=\(.fail)"' "$OUT_DIR/stress.json" 2>/dev/null)

printf "  %-12s %s\n" "L1 app:" "$L1"
printf "  %-12s %s\n" "L2 network:" "$L2"
printf "  %-12s %s\n" "L3 system:" "$L3"
echo ""

# ============================================================
# Part 7: 其他场景速览
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 7: 其他场景速览"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cat <<'TEXT'
  ┌─ 场景 A: 网络延迟 (需要 podman)
  │
  │  # 启动 MinIO 容器
  │  ./test/scenario/02-network-lag.sh
  │
  │  # 或在已有接口上加延迟
  │  tc qdisc add dev eth0 root netem delay 200ms
  │  stordiag doctor --endpoint <host>:9000
  │  tc qdisc del dev eth0 root
  │
  │  预期: L2 tcp_connect↑ 上百 ms，L1 跟着涨，L3 正常
  │  根因: L2 网络层
  │
  ├─ 场景 B: 服务宕机 (需要 podman)
  │
  │  ./test/scenario/04-service-down.sh
  │
  │  预期: L1 ping FAIL，L2 tcp 正常或 wall，L3 正常
  │  根因: L1+L2 应用层
  │
  ├─ 场景 C: 数据损坏 (无依赖)
  │
  │  ./test/scenario/03-data-corrupt.sh
  │
  │  预期: 篡改后文件 sha256 与预期不符
  │  根因: L1 data_integrity FAIL
  │
  └─ 场景 D: 一键所有场景
     ./test/run.sh
TEXT
echo ""

# ============================================================
# Part 8: 清理
# ============================================================
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
say "Part 8: 清理"
say "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
rm -rf "$TEST_DIR"
say "测试数据已清理"
echo ""

say "================================================"
say "DEMO 结束"
say "================================================"
echo ""
cat <<'TEXT'
  所有报告保存在:
    $OUT_DIR = $OUT_DIR/
    ├── baseline.json     ← 健康基线
    └── stress.json       ← IO 压力下

  交互式探索:
    jq '.' "$OUT_DIR/baseline.json"          # 看完整 JSON 结构
    jq '.Layers[].probes[] | select(.status=="WARN")' "$OUT_DIR/stress.json"  # 只看 WARN
    jq '.Layers[].probes[] | select(.status=="FAIL")' "$OUT_DIR/stress.json"  # 只看 FAIL
    ./stordiag diff "$OUT_DIR/baseline.json" "$OUT_DIR/stress.json"           # 对比

  推荐学习路径:
    1. 先跑 00-demo.sh 熟悉输出格式
    2. 跑 03-data-corrupt.sh 看最简单的 FAIL 场景
    3. 跑 01-disk-io.sh 看 L3 系统层变化
    4. 用 stordiag doctor --prometheus 接入监控
    5. 用 stordiag watch 排查间歇性问题
TEXT
