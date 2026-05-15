# stordiag — Storage Diagnostics Toolkit / 存储诊断工具箱

> **Single binary, zero-dependency** distributed storage diagnostics.
> Supports **S3-compatible object storage** and **POSIX filesystem** with three-layer hierarchical diagnosis.
>
> **单二进制、零依赖**的分布式存储诊断工具箱。支持 **S3 兼容对象存储** 和 **POSIX 文件系统** 的三层分层诊断。

## Architecture / 架构

| Layer / 层 | Name / 名称 | Probes / 探针 | Data Source / 数据来源 |
|---|---|---|---|
| **L1** | Application / 应用层 | ping / bench (read/write) / data integrity | Storage API end-to-end |
| **L2** | Network / 网络层 | DNS lookup / TCP connect / TLS handshake | `net` + `crypto/tls` |
| **L3** | System / 系统层 | disk IOPS+await / memory / CPU iowait / Pressure Stall | `/proc/*` |

Diagnostics aggregated by layer / 诊断结果按层聚合：

```
  Application  OK=3 WARN=0 FAIL=0
  Network      OK=2 WARN=0 FAIL=0
  System       OK=5 WARN=0 FAIL=0
```

## Quick Start / 快速开始

### Build / 构建

```bash
go build -o stordiag .
# or
make build
```

### POSIX (Local Filesystem / 本地文件系统)

```bash
# Full three-layer diagnosis / 三层全量诊断
./stordiag doctor --endpoint /tmp/stordiag_test

# System layer only / 只看系统层
./stordiag doctor --endpoint /tmp/stordiag_test --layers system

# Single layer / 单层独立执行
./stordiag layers system --endpoint /tmp/stordiag_test

# JSON output
./stordiag doctor --json --endpoint /tmp/stordiag_test
```

### S3 / MinIO

```bash
export STORDIAG_ENDPOINT=192.168.1.100:9000
export STORDIAG_ACCESS_KEY=minioadmin
export STORDIAG_SECRET_KEY=minioadmin
export STORDIAG_BUCKET=stordiag

./stordiag doctor
./stordiag layers network
./stordiag bench write --size=4M --concurrency=4
```

## Complete Command Reference / 完整命令手册

### Global Flags / 全局标志

Applies to all subcommands via CLI flags, environment variables, or config file.
适用于所有子命令，可通过 CLI flag、环境变量、或配置文件设置。

| Flag | Env / 环境变量 | Default / 默认值 | Description / 说明 |
|---|---|---|---|
| `--endpoint` | `STORDIAG_ENDPOINT` | `localhost:9000` | Storage endpoint. POSIX: path (e.g. `/data`), S3: `host:port` |
| `--access-key` | `STORDIAG_ACCESS_KEY` | `minioadmin` | S3 access key. Ignored for POSIX. / POSIX 忽略 |
| `--secret-key` | `STORDIAG_SECRET_KEY` | `minioadmin` | S3 secret key. Ignored for POSIX. / POSIX 忽略 |
| `--bucket` | `STORDIAG_BUCKET` | `stordiag` | S3 bucket name. Ignored for POSIX. / POSIX 忽略 |
| `--secure` | `STORDIAG_SECURE` | `false` | Enable TLS for S3 / S3 启用 TLS |
| `--timeout` | `STORDIAG_TIMEOUT` | `30` | Operation timeout (seconds) / 操作超时（秒） |
| `--json` | — | `false` | JSON output / JSON 格式输出 |
| `--config` | — | `.stordiag.yaml` | Config file path / 配置文件路径 |

Auto-detection: endpoint starting with `/` or `.` → POSIX path, otherwise → S3 `host:port`.
端点自动检测：以 `/` 或 `.` 开头视为 POSIX 路径，否则视为 S3 `host:port`。

---

### `doctor` — Full Three-Layer Diagnosis / 全量三层诊断

Runs all available probes against the storage backend and outputs an aggregated report.
对存储后端执行所有可用层的诊断，输出聚合报告。

```
stordiag doctor [flags]
```

#### Flags

| Flag | Default / 默认值 | Description / 说明 |
|---|---|---|
| `--layers` | `all` | Which layers: `all` / `app` / `network` / `system` |

#### Probe List / 探针清单

| Layer / 层 | Probe / 探针 | Measurement / 测量内容 |
|---|---|---|
| L1:app | `ping` | Connectivity + RTT / 存储端点连通性 + 往返延迟 |
| L1:app | `bench_write` | 4MiB write throughput + p99 latency / 4MiB 写入吞吐 + p99 延迟 |
| L1:app | `data_integrity` | 512KiB end-to-end write→read→sha256 / 512KiB 端到端写→读→sha256 校验 |
| L2:network | `dns_lookup` | DNS resolution latency / DNS 解析延迟 |
| L2:network | `tcp_connect` | TCP connect latency / TCP 建连延迟 |
| L2:network | `tls_handshake` | TLS handshake latency + protocol version / TLS 握手延迟 + 协议版本 |
| L3:system | `disk_*` | Block device IOPS / await / queue depth / 各块设备 IOPS / await / 队列深度 |
| L3:system | `memory_available` | Available memory (MiB) / 可用内存 (MiB) |
| L3:system | `pressure_cpu` | CPU Pressure Stall (avg60) |
| L3:system | `pressure_io` | IO Pressure Stall (avg60) |
| L3:system | `cpu_iowait` | /proc/stat iowait ticks |

#### Examples / 示例

```bash
# Full three-layer / 全量三层
stordiag doctor --endpoint /data

# Application only / 只看应用层
stordiag doctor --layers app --endpoint /data

# CI integration / CI 集成
stordiag doctor --json --endpoint /data | jq '.Summary'
```

#### Output Example / 输出示例（text）

```
=== Doctor Report: /data (posix) ===
  Timestamp:  2026-05-15T17:29:00+08:00
  Summary:    PASS

--- Application [L1:app]  OK=3  WARN=0  FAIL=0 ---
Probe           Status  Latency    Value                        Detail
-----           ------  -------    -----                        ------
ping            OK      2.4µs      reachable
bench_write     OK      953µs      4121.0 MB/s  p99=1.0ms
data_integrity  OK      -          checksums match              sha256

--- Network [L2:network]  OK=0  WARN=0  FAIL=0 ---
Probe    Status  Latency  Value                          Detail
-----    ------  -------  -----                          ------
network  N/A     -        local filesystem — no network

--- System [L3:system]  OK=5  WARN=0  FAIL=0 ---
Probe             Status  Latency  Value                                       Detail
-----             ------  -------  -----                                       ------
disk_nvme0n1      OK      -        iops=120 r_await=0.1ms w_await=0.3ms queue=0  /proc/diskstats
memory_available  OK      -        26350 MiB                                   /proc/meminfo
pressure_cpu      OK      -        avg60=0.0%                                  /proc/pressure/cpu
pressure_io       OK      -        avg60=0.2%                                  /proc/pressure/io
cpu_iowait        OK      -        iowait=83350.0 tick                         /proc/stat
```

---

### `layers` — Single Layer Execution / 单层独立执行

Run probes for a single layer only. Useful for focused troubleshooting.
只执行指定层的探针，适合聚焦排查。

```
stordiag layers <app|network|system> [flags]
```

#### Arguments / 参数

| Arg / 参数 | Description / 说明 |
|---|---|
| `app` | Application layer: health + bench + data integrity / 应用层 |
| `network` | Network layer: DNS/TCP/TLS / 网络层 |
| `system` | System layer: disk/memory/CPU/PSI / 系统层 |

#### Examples / 示例

```bash
# System layer only / 只看系统层
stordiag layers system --endpoint /data

# Network layer JSON output / 网络层 JSON 输出
stordiag layers network --endpoint s3.example.com:443 --json

# Application layer (quick check) / 应用层快速巡检
stordiag layers app --endpoint minio-cluster:9000
```

---

### `health` — Health Check / 健康检查

Quickly check storage endpoint connectivity and basic status.
快速检查存储端点的连通性和基本状态。

```
stordiag health [target] [flags]
```

#### Arguments / 参数

| Arg / 参数 | Description / 说明 |
|---|---|
| `target` | Optional, overrides `--endpoint` / 可选，覆盖 `--endpoint` |

#### Probes / 探针

- `root_stat` — Root path / bucket status / 根路径/桶状态
- `list_objects` — List objects to verify read permission / 列出对象验证读权限

#### Examples / 示例

```bash
# Basic health check / 基本健康检查
stordiag health --endpoint /mnt/data

# S3 endpoint health
stordiag health --endpoint 10.0.0.1:9000

# JSON output (monitoring integration) / 适合监控集成
stordiag health --json --endpoint /data
```

#### Output Example / 输出示例

```
=== Health Check: /data (posix) ===
  Timestamp:  2026-05-15T17:13:02+08:00
  Reachable:  true
  Latency:    6µs

Check         Status  Value
-----         ------  -----
root_stat     OK      920 B
list_objects  OK      44 objects
```

---

### `bench` — Performance Benchmark / 性能压测

Run read/write performance benchmarks against the storage backend.
对存储后端执行读写性能基准测试，输出吞吐和延迟分布。

```
stordiag bench <read|write> [flags]
```

#### Arguments / 参数

| Arg / 参数 | Description / 说明 |
|---|---|
| `read` | Read benchmark (write then read) / 读性能测试 |
| `write` | Write benchmark / 写性能测试 |

#### Flags

| Flag | Default / 默认值 | Description / 说明 |
|---|---|---|
| `--size` | `4194304` (4 MiB) | IO size in bytes / 每次 IO 的数据大小 |
| `--concurrency` | `4` | Concurrent operations / 并发操作数 |
| `--samples` | `10` | Sample count / 采样次数 |

#### Examples / 示例

```bash
# 4MiB write, 4 concurrency
stordiag bench write --size=4M --concurrency=4

# 64KiB small file write (high IOPS scenario)
stordiag bench write --size=64K --concurrency=8 --samples=100

# Large file read
stordiag bench read --size=64M --concurrency=2 --samples=5

# S3 scenario
stordiag bench write --size=4M --concurrency=16 --endpoint minio:9000
```

#### Output Example / 输出示例

```
=== Bench: /data (posix) ===
Metric       Value
------       -----
Operation    write
Size         4.0 MiB
Concurrency  4
Duration     1.2ms
Throughput   13500.20 MB/s
Avg Latency  1.05ms
P99 Latency  1.42ms
Errors       0
```

#### Parameter Guide / 调参建议

| Scenario / 场景 | size | concurrency | samples |
|---|---|---|---|
| Throughput baseline / 吞吐基准 | 4MiB | 4-8 | 10-30 |
| IOPS baseline (small file) / IOPS 基准 | 4KiB | 16-64 | 100-500 |
| Large sequential / 大块顺序读写 | 64MiB | 1-2 | 5-10 |
| Extreme stress / 极端压力 | 1MiB | 64-128 | 50 |

---

### `verify` — Data Integrity / 数据完整性校验

End-to-end data integrity verification: write known data → read back → SHA256/MD5 comparison. Detects silent data corruption, bit rot, and network transfer errors.
端到端数据完整性检测：写入已知数据 → 读出 → SHA256/MD5 比对。检测静默数据损坏、bit rot、网络传输错误。

```
stordiag verify <path> [flags]
```

#### Arguments / 参数

| Arg / 参数 | Description / 说明 |
|---|---|
| `path` | Path on the storage backend / 在存储后端上的路径 |

#### Flags

| Flag | Default / 默认值 | Description / 说明 |
|---|---|---|
| `--algo` | `sha256` | Algorithm: `sha256` or `md5` / 校验算法 |

#### Examples / 示例

```bash
# Default sha256
stordiag verify /backup/db.sql --endpoint /data

# MD5 quick check
stordiag verify test.dat --algo md5

# S3 object verification
stordiag verify important.bin --endpoint s3.company.com
```

#### Output Example / 输出示例

```
=== Verify: /data/backup/db.sql ===
  Algorithm:  sha256
  Remote:     a1b2c3d4e5f6...
  Local:      a1b2c3d4e5f6...
  Result:     OK (checksums match)
```

On corruption / 损坏时：

```
=== Verify: /data/backup/db.sql ===
  Algorithm:  sha256
  Remote:     09d7f123ea11...
  Local:      d1ec62d006da...
  Result:     FAIL (checksums mismatch!)
```

## Output Formats / 输出格式

### Text (Default / 默认)

Terminal-friendly, tabwriter-aligned, grouped by layer.
终端友好，带 tabwriter 对齐和层分组。

### JSON (`--json`)

Structured output, suitable for CI/CD integration and programmatic parsing.
结构化输出，适合 CI/CD 集成和程序解析：

```json
{
  "Target": "/tmp",
  "Type": "posix",
  "Layers": [
    {
      "layer": "L1:app",
      "label": "Application",
      "ok": 3, "warn": 0, "fail": 0,
      "probes": [
        { "name": "ping",         "status": "OK", "latency": "3.1µs",  "value": "reachable" },
        { "name": "bench_write",  "status": "OK", "latency": "1.2ms",  "value": "3800.1 MB/s  p99=1.4ms" },
        { "name": "data_integrity","status": "OK", "latency": "0s",    "value": "checksums match", "detail": "sha256" }
      ]
    }
  ],
  "Summary": "PASS",
  "Timestamp": "2026-05-15T17:29:06+08:00"
}
```

## Fault Diagnosis Matrix / 故障场景诊断矩阵

| Scenario / 场景 | L1 Application / 应用层 | L2 Network / 网络层 | L3 System / 系统层 | Root Cause / 根因定位 |
|---|---|---|---|---|
| Normal / 正常 | Low latency, high throughput | TCP <1ms | Normal IOPS | — |
| Network latency / 网络延迟 | Latency spikes | TCP connect↑ | Normal IOPS | **L2** |
| Packet loss / 网络丢包 | Throughput drops | TCP retransmit/timeout | Normal IOPS | **L2** |
| Busy disk / 磁盘繁忙 | Latency ↑, throughput ↓ | Network normal | await↑ iops↓ pressure_io↑ | **L3** |
| Server OOM / 服务端 OOM | Connection reset | TCP normal → app timeout | mem_avail↓ | **L1+L3** |
| CPU saturation / CPU 打满 | Latency ↑ | Network normal | cpu_iowait↑ | **L3** |

### Fault Simulation / 故障模拟

```bash
# Simulate IO pressure / 模拟 IO 压力
stress-ng --hdd 4 --hdd-bytes 4G --timeout 60s &
./stordiag doctor --layers system

# Simulate memory pressure / 模拟内存压力
stress-ng --vm 2 --vm-bytes 2G --timeout 60s &
./stordiag layers system

# Simulate network latency (requires MinIO Docker) / 模拟网络延迟
docker exec minio tc qdisc add dev eth0 root netem delay 100ms
./stordiag layers network
```

## Configuration / 配置方式

Priority: **CLI flags > Environment variables / 环境变量 > Config file / 配置文件 > Defaults / 默认值**

### Environment Variables / 环境变量

All CLI flags can be set via `STORDIAG_` prefixed env vars:
所有 CLI flags 均可通过 `STORDIAG_` 前缀环境变量设置：

```bash
export STORDIAG_ENDPOINT=localhost:9000
export STORDIAG_ACCESS_KEY=admin
export STORDIAG_SECRET_KEY=secret
export STORDIAG_BUCKET=data
export STORDIAG_SECURE=true
export STORDIAG_TIMEOUT=60
```

### Config File / 配置文件

YAML format `.stordiag.yaml` or `.stordiag.json` in current directory, or `--config` specified:

```yaml
endpoint: localhost:9000
access-key: admin
secret-key: secret
bucket: data
secure: false
```

## Integration with eBPF / 与 eBPF 的分层配合

```
stordiag doctor --json              # ① Periodic check: find "which layer" / 定期巡检
        ↓ alert / 告警
stordiag layers system              # ② Focus on specific layer / 聚焦特定层
        ↓ confirmed L3 disk issue / 确认是 L3 磁盘问题
sudo biolatency                      # ③ eBPF deep dive: IO latency distribution / IO 延迟分布
sudo fileslower 10                   # ④ Slow file IO tracing / 文件级慢 IO 追踪
sudo trace block_rq_issue           # ⑤ Block layer IO events / 块层 IO 事件
```

| Tool / 工具 | Focus / 定位 | Deployment / 部署 | Precision / 精度 | Use Case / 场景 |
|---|---|---|---|---|
| stordiag | End-to-end from business perspective / 业务视角端到端 | Zero dependency, normal user / 零依赖, 普通用户 | ms/µs | Routine check / CI / 常态化巡检/CI |
| eBPF (bcc/bpftrace) | Kernel-level layer-by-layer / 内核视角逐层分解 | root, specific kernel / root, 特定内核 | ns | Deep dive / tuning / 深度排查/调优 |

stordiag for **first-line screening**, eBPF for **on-demand deep dive**. They complement each other.
stordiag 做**第一道筛选**，eBPF 做**按需深挖**。两者互补。

## Build / 构建

```bash
make build                 # Current platform / 当前平台
make cross                 # Cross-compile all platforms / 交叉编译所有平台
make fmt                   # go fmt
make lint                  # go vet
make clean

# Single platform cross-compile / 单平台交叉编译
GOOS=linux GOARCH=arm64 go build -o stordiag-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -o stordiag-darwin-amd64 .
```

## License

MIT
