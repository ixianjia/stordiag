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

## Command Reference / 命令手册

### Global Flags / 全局标志

| Flag | Env / 环境变量 | Default / 默认值 | Description / 说明 |
|---|---|---|---|
| `--endpoint` | `STORDIAG_ENDPOINT` | `localhost:9000` | Storage endpoint. POSIX: path, S3: `host:port` |
| `--access-key` | `STORDIAG_ACCESS_KEY` | `minioadmin` | S3 access key. Ignored for POSIX. / POSIX 忽略 |
| `--secret-key` | `STORDIAG_SECRET_KEY` | `minioadmin` | S3 secret key. Ignored for POSIX. / POSIX 忽略 |
| `--bucket` | `STORDIAG_BUCKET` | `stordiag` | S3 bucket name. Ignored for POSIX. / POSIX 忽略 |
| `--secure` | `STORDIAG_SECURE` | `false` | Enable TLS for S3 / S3 启用 TLS |
| `--timeout` | `STORDIAG_TIMEOUT` | `30` | Operation timeout (seconds) / 操作超时（秒） |
| `--json` | — | `false` | JSON output / JSON 格式输出 |
| `--config` | — | `.stordiag.yaml` | Config file path / 配置文件路径 |

Auto-detection: endpoint starting with `/` or `.` → POSIX path, otherwise → S3 `host:port`.
端点自动检测：以 `/` 或 `.` 开头视为 POSIX 路径，否则视为 S3 `host:port`。

All flags can also be set via `STORDIAG_` env vars or a YAML/JSON config file. Priority: **CLI > Env > Config > Default**.

---

### `doctor` — Full Three-Layer Diagnosis / 全量三层诊断

```
stordiag doctor [flags]
```

**Flags**: `--layers` (default `all`) — `all` / `app` / `network` / `system`

Probes / 探针清单:

| Layer / 层 | Probe / 探针 | Measurement / 测量内容 |
|---|---|---|
| L1:app | `ping` | Connectivity + RTT / 连通性 + 往返延迟 |
| L1:app | `bench_write` | 4MiB write throughput + p99 latency |
| L1:app | `data_integrity` | 512KiB end-to-end write→read→sha256 |
| L2:network | `dns_lookup` | DNS resolution latency / DNS 解析延迟 |
| L2:network | `tcp_connect` | TCP connect latency / TCP 建连延迟 |
| L2:network | `tls_handshake` | TLS handshake latency + protocol version |
| L3:system | `disk_*` | Block device IOPS / await / queue depth |
| L3:system | `memory_available` | Available memory (MiB) / 可用内存 |
| L3:system | `pressure_cpu` | CPU Pressure Stall (avg60) |
| L3:system | `pressure_io` | IO Pressure Stall (avg60) |
| L3:system | `cpu_iowait` | /proc/stat iowait ticks |

```bash
# Full three-layer / 全量三层
stordiag doctor --endpoint /data

# Application layer only / 只看应用层
stordiag doctor --layers app --endpoint /data

# CI integration / CI 集成
stordiag doctor --json --endpoint /data | jq '.Summary'
```

Output example / 输出示例:

```
=== Doctor Report: /data (posix) ===
  Timestamp:  2026-05-15T17:29:00+08:00
  Summary:    PASS

--- Application [L1:app]  OK=3  WARN=0  FAIL=0 ---
Probe           Status  Latency    Value                        Detail
ping            OK      2.4µs      reachable
bench_write     OK      953µs      4121.0 MB/s  p99=1.0ms
data_integrity  OK      -          checksums match              sha256

--- Network [L2:network]  OK=0  WARN=0  FAIL=0 ---
Probe    Status  Latency  Value                          Detail
network  N/A     -        local filesystem — no network

--- System [L3:system]  OK=5  WARN=0  FAIL=0 ---
Probe             Status  Latency  Value
disk_nvme0n1      OK      -        iops=120 r_await=0.1ms w_await=0.3ms
memory_available  OK      -        26350 MiB
pressure_cpu      OK      -        avg60=0.0%
pressure_io       OK      -        avg60=0.2%
cpu_iowait        OK      -        iowait=83350.0 tick
```

JSON output is also supported via `--json` for CI/CD integration.

---

### `layers` — Single Layer Execution / 单层独立执行

```
stordiag layers <app|network|system> [flags]
```

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

```
stordiag health [target] [flags]
```

Quickly check connectivity and basic status. Probes: `root_stat`, `list_objects`.
快速检查连通性和基本状态。

```bash
stordiag health --endpoint /mnt/data
stordiag health --endpoint 10.0.0.1:9000
stordiag health --json --endpoint /data
```

---

### `bench` — Performance Benchmark / 性能压测

```
stordiag bench <read|write> [flags]
```

**Flags**: `--size` (default 4MiB), `--concurrency` (default 4), `--samples` (default 10)

| Scenario / 场景 | size | concurrency | samples |
|---|---|---|---|
| Throughput baseline / 吞吐基准 | 4MiB | 4-8 | 10-30 |
| IOPS baseline / IOPS 基准 | 4KiB | 16-64 | 100-500 |
| Large sequential / 大块顺序读写 | 64MiB | 1-2 | 5-10 |
| Extreme stress / 极端压力 | 1MiB | 64-128 | 50 |

```bash
stordiag bench write --size=4M --concurrency=4
stordiag bench write --size=64K --concurrency=8 --samples=100
stordiag bench read --size=64M --concurrency=2 --samples=5
stordiag bench write --size=4M --concurrency=16 --endpoint minio:9000
```

---

### `verify` — Data Integrity / 数据完整性校验

End-to-end verification: write → read → SHA256/MD5 comparison. Detects silent data corruption and bit rot.
端到端数据完整性检测：写入 → 读出 → SHA256/MD5 比对。检测静默数据损坏和 bit rot。

```
stordiag verify <path> [flags]
```

**Flags**: `--algo` (default `sha256`, also supports `md5`)

```bash
stordiag verify /backup/db.sql --endpoint /data
stordiag verify test.dat --algo md5
stordiag verify important.bin --endpoint s3.company.com
```

## Fault Diagnosis Matrix / 故障场景诊断矩阵

| Scenario / 场景 | L1 Application | L2 Network | L3 System | Root Cause / 根因 |
|---|---|---|---|---|
| Normal / 正常 | Low latency, high throughput | TCP <1ms | Normal IOPS | — |
| Network latency / 网络延迟 | Latency spikes | TCP connect↑ | Normal IOPS | **L2** |
| Packet loss / 网络丢包 | Throughput drops | TCP retransmit/timeout | Normal IOPS | **L2** |
| Busy disk / 磁盘繁忙 | Latency ↑, throughput ↓ | Network normal | await↑ iops↓ | **L3** |
| Server OOM / 服务端 OOM | Connection reset | TCP normal → app timeout | mem_avail↓ | **L1+L3** |
| CPU saturation / CPU 打满 | Latency ↑ | Network normal | cpu_iowait↑ | **L3** |

## License

MIT
