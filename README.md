# stordiag — 存储诊断工具箱

[**English**](README.en.md) | 中文

> **单二进制、零依赖**的分布式存储诊断工具箱。
> 支持 **S3 兼容对象存储** 和 **POSIX 文件系统** 的三层分层诊断。

## 架构

| 层 | 名称 | 探针 | 数据来源 |
|---|---|---|---|
| **L1** | 应用层 | ping / bench (read/write) / data integrity | 存储 API 端到端 |
| **L2** | 网络层 | DNS lookup / TCP connect / TLS handshake | `net` + `crypto/tls` |
| **L3** | 系统层 | disk IOPS+await / memory / CPU iowait / Pressure Stall | `/proc/*` |

诊断结果按层聚合：

```
  Application  OK=3 WARN=0 FAIL=0
  Network      OK=2 WARN=0 FAIL=0
  System       OK=5 WARN=0 FAIL=0
```

## 快速开始

### 构建

```bash
go build -o stordiag .
# 或
make build
```

### POSIX（本地文件系统）

```bash
# 三层全量诊断
./stordiag doctor --endpoint /tmp/stordiag_test

# 只看系统层
./stordiag doctor --endpoint /tmp/stordiag_test --layers system

# 单层独立执行
./stordiag layers system --endpoint /tmp/stordiag_test

# JSON 输出
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

## 命令手册

### 全局标志

| Flag | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `--endpoint` | `STORDIAG_ENDPOINT` | `localhost:9000` | 存储端点。POSIX 传路径，S3 传 `host:port` |
| `--access-key` | `STORDIAG_ACCESS_KEY` | `minioadmin` | S3 访问密钥。POSIX 忽略 |
| `--secret-key` | `STORDIAG_SECRET_KEY` | `minioadmin` | S3 秘密密钥。POSIX 忽略 |
| `--bucket` | `STORDIAG_BUCKET` | `stordiag` | S3 桶名。POSIX 忽略 |
| `--secure` | `STORDIAG_SECURE` | `false` | S3 启用 TLS |
| `--timeout` | `STORDIAG_TIMEOUT` | `30` | 操作超时（秒） |
| `--json` | — | `false` | JSON 格式输出 |
| `--config` | — | `.stordiag.yaml` | 配置文件路径 |

端点自动检测：以 `/` 或 `.` 开头视为 POSIX 路径，否则视为 S3 `host:port`。

所有 flag 也可通过 `STORDIAG_` 环境变量或 YAML/JSON 配置文件设置。优先级：**CLI > 环境变量 > 配置文件 > 默认值**

---

### `doctor` — 全量三层诊断

```
stordiag doctor [flags]
```

**Flags**: `--layers`（默认 `all`）— `all` / `app` / `network` / `system`

探针清单：

| 层 | 探针 | 测量内容 |
|---|---|---|
| L1:app | `ping` | 存储端点连通性 + 往返延迟 |
| L1:app | `bench_write` | 4MiB 写入吞吐 + p99 延迟 |
| L1:app | `data_integrity` | 512KiB 端到端写→读→sha256 校验 |
| L2:network | `dns_lookup` | DNS 解析延迟 |
| L2:network | `tcp_connect` | TCP 建连延迟 |
| L2:network | `tls_handshake` | TLS 握手延迟 + 协议版本 |
| L3:system | `disk_*` | 各块设备 IOPS / await / 队列深度 |
| L3:system | `memory_available` | 可用内存 (MiB) |
| L3:system | `pressure_cpu` | CPU Pressure Stall (avg60) |
| L3:system | `pressure_io` | IO Pressure Stall (avg60) |
| L3:system | `cpu_iowait` | /proc/stat iowait ticks |

```bash
# 全量三层
stordiag doctor --endpoint /data

# 只看应用层
stordiag doctor --layers app --endpoint /data

# CI 集成
stordiag doctor --json --endpoint /data | jq '.Summary'
```

输出示例：

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

也支持 `--json` 输出，适合 CI/CD 集成。

---

### `layers` — 单层独立执行

```
stordiag layers <app|network|system> [flags]
```

```bash
# 只看系统层
stordiag layers system --endpoint /data

# 网络层 JSON 输出
stordiag layers network --endpoint s3.example.com:443 --json

# 应用层（快速巡检）
stordiag layers app --endpoint minio-cluster:9000
```

---

### `health` — 健康检查

```
stordiag health [target] [flags]
```

快速检查连通性和基本状态。探针：`root_stat`、`list_objects`。

```bash
stordiag health --endpoint /mnt/data
stordiag health --endpoint 10.0.0.1:9000
stordiag health --json --endpoint /data
```

---

### `bench` — 性能压测

```
stordiag bench <read|write> [flags]
```

**Flags**: `--size`（默认 4MiB）、`--concurrency`（默认 4）、`--samples`（默认 10）

| 场景 | size | concurrency | samples |
|---|---|---|---|
| 吞吐基准 | 4MiB | 4-8 | 10-30 |
| IOPS 基准（小文件） | 4KiB | 16-64 | 100-500 |
| 大块顺序读写 | 64MiB | 1-2 | 5-10 |
| 极端压力 | 1MiB | 64-128 | 50 |

```bash
stordiag bench write --size=4M --concurrency=4
stordiag bench write --size=64K --concurrency=8 --samples=100
stordiag bench read --size=64M --concurrency=2 --samples=5
stordiag bench write --size=4M --concurrency=16 --endpoint minio:9000
```

---

### `verify` — 数据完整性校验

端到端数据完整性检测：写入 → 读出 → SHA256/MD5 比对。检测静默数据损坏和 bit rot。

```
stordiag verify <path> [flags]
```

**Flags**: `--algo`（默认 `sha256`，也支持 `md5`）

```bash
stordiag verify /backup/db.sql --endpoint /data
stordiag verify test.dat --algo md5
stordiag verify important.bin --endpoint s3.company.com
```

## 故障场景诊断矩阵

| 场景 | L1 应用层 | L2 网络层 | L3 系统层 | 根因定位 |
|---|---|---|---|---|
| 正常 | 低延迟高吞吐 | TCP <1ms | IOPS 正常 | — |
| 网络延迟 | 延迟波动 | TCP connect↑ | IOPS 正常 | **L2** |
| 网络丢包 | 吞吐骤降 | TCP 重传/超时 | IOPS 正常 | **L2** |
| 磁盘繁忙 | 延迟升高吞吐降 | 网络正常 | await↑ iops↓ | **L3** |
| 服务端 OOM | 连接重置 | TCP 正常→应用超时 | mem_avail↓ | **L1+L3** |
| CPU 打满 | 延迟升高 | 网络正常 | cpu_iowait↑ | **L3** |

## License

MIT
