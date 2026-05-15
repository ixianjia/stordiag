# stordiag — Storage Diagnostics Toolkit

单二进制、零依赖的分布式存储诊断工具箱。支持 **S3 兼容对象存储** 和 **POSIX 文件系统** 的三层分层诊断。

## 架构

| 层 | 名称 | 探针 | 数据来源 |
|---|---|---|---|
| **L1** | 应用层 | ping / bench (read/write) / data integrity | 存储 API 端到端 |
| **L2** | 网络层 | DNS lookup / TCP connect / TLS handshake | `net` + `crypto/tls` |
| **L3** | 系统层 | disk IOPS+await / memory / CPU iowait / Pressure Stall | `/proc/*` |

诊断结果按层聚合，一目了然：

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

## 完整命令手册

### 全局标志

适用于所有子命令，可通过 CLI flag、环境变量、或配置文件设置。

| Flag | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `--endpoint` | `STORDIAG_ENDPOINT` | `localhost:9000` | 存储端点。POSIX 传路径 (如 `/data`)，S3 传 `host:port` |
| `--access-key` | `STORDIAG_ACCESS_KEY` | `minioadmin` | S3 访问密钥。POSIX 忽略 |
| `--secret-key` | `STORDIAG_SECRET_KEY` | `minioadmin` | S3 秘密密钥。POSIX 忽略 |
| `--bucket` | `STORDIAG_BUCKET` | `stordiag` | S3 桶名。POSIX 忽略 |
| `--secure` | `STORDIAG_SECURE` | `false` | S3 启用 TLS |
| `--timeout` | `STORDIAG_TIMEOUT` | `30` | 操作超时（秒） |
| `--json` | — | `false` | JSON 格式输出 |
| `--config` | — | `.stordiag.yaml` | 配置文件路径 |

端点自动检测：以 `/` 或 `.` 开头视为 POSIX 路径，否则视为 S3 `host:port`。

---

### `doctor` — 全量三层诊断

对存储后端执行所有可用层的诊断，输出聚合报告。

```
stordiag doctor [flags]
```

#### Flags

| Flag | 默认值 | 说明 |
|---|---|---|
| `--layers` | `all` | 运行哪些层：`all` / `app` / `network` / `system` |

#### 探针清单

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

#### 示例

```bash
# 全量三层
stordiag doctor --endpoint /data

# 只看应用层
stordiag doctor --layers app --endpoint /data

# CI 集成
stordiag doctor --json --endpoint /data | jq '.Summary'
```

#### 输出示例（text）

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

### `layers` — 单层独立执行

只执行指定层的探针，适合聚焦排查。

```
stordiag layers <app|network|system> [flags]
```

#### 参数

| 参数 | 说明 |
|---|---|
| `app` | 应用层：health + bench + data integrity |
| `network` | 网络层：DNS/TCP/TLS 分解 |
| `system` | 系统层：磁盘/内存/CPU/PSI |

#### 示例

```bash
# 只看系统层
stordiag layers system --endpoint /data

# 网络层 JSON 输出
stordiag layers network --endpoint s3.example.com:443 --json

# 应用层（作为快速巡检）
stordiag layers app --endpoint minio-cluster:9000
```

---

### `health` — 健康检查

快速检查存储端点的连通性和基本状态。

```
stordiag health [target] [flags]
```

#### 参数

| 参数 | 说明 |
|---|---|
| `target` | 可选，覆盖 `--endpoint` |

#### 探针

- `root_stat` — 根路径/桶状态
- `list_objects` — 列出对象验证读权限

#### 示例

```bash
# 基本健康检查
stordiag health --endpoint /mnt/data

# S3 端点健康
stordiag health --endpoint 10.0.0.1:9000

# JSON 输出 (适合监控集成)
stordiag health --json --endpoint /data
```

#### 输出示例

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

### `bench` — 性能压测

对存储后端执行读写性能基准测试，输出吞吐和延迟分布。

```
stordiag bench <read|write> [flags]
```

#### 参数

| 参数 | 说明 |
|---|---|
| `read` | 读性能测试（先写入后读出） |
| `write` | 写性能测试 |

#### Flags

| Flag | 默认值 | 说明 |
|---|---|---|
| `--size` | `4194304` (4 MiB) | 每次 IO 的数据大小（字节） |
| `--concurrency` | `4` | 并发操作数 |
| `--samples` | `10` | 采样次数 |

#### 示例

```bash
# 4MiB 写入 4 并发
stordiag bench write --size=4M --concurrency=4

# 64KiB 小文件写入 (高 IOPS 场景)
stordiag bench write --size=64K --concurrency=8 --samples=100

# 大文件读取
stordiag bench read --size=64M --concurrency=2 --samples=5

# S3 场景
stordiag bench write --size=4M --concurrency=16 --endpoint minio:9000
```

#### 输出示例

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

#### 调参建议

| 场景 | size | concurrency | samples |
|---|---|---|---|
| 吞吐基准 | 4MiB | 4-8 | 10-30 |
| IOPS 基准 (小文件) | 4KiB | 16-64 | 100-500 |
| 大块顺序读写 | 64MiB | 1-2 | 5-10 |
| 极端压力 | 1MiB | 64-128 | 50 |

---

### `verify` — 数据完整性校验

端到端数据完整性检测：写入已知数据 → 读出 → SHA256/MD5 比对。检测静默数据损坏、bit rot、网络传输错误。

```
stordiag verify <path> [flags]
```

#### 参数

| 参数 | 说明 |
|---|---|
| `path` | 在存储后端上的路径 |

#### Flags

| Flag | 默认值 | 说明 |
|---|---|---|
| `--algo` | `sha256` | 校验算法：`sha256` 或 `md5` |

#### 示例

```bash
# 默认 sha256
stordiag verify /backup/db.sql --endpoint /data

# MD5 快速校验
stordiag verify test.dat --algo md5

# S3 对象校验
stordiag verify important.bin --endpoint s3.company.com
```

#### 输出示例

```
=== Verify: /data/backup/db.sql ===
  Algorithm:  sha256
  Remote:     a1b2c3d4e5f6...
  Local:      a1b2c3d4e5f6...
  Result:     OK (checksums match)
```

损坏时：

```
=== Verify: /data/backup/db.sql ===
  Algorithm:  sha256
  Remote:     09d7f123ea11...
  Local:      d1ec62d006da...
  Result:     FAIL (checksums mismatch!)
```

## 输出格式

### Text（默认）

终端友好，带 tabwriter 对齐和层分组。

### JSON（`--json`）

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

## 故障场景诊断矩阵

| 场景 | L1 应用层 | L2 网络层 | L3 系统层 | 根因定位 |
|---|---|---|---|---|
| 正常 | 低延迟高吞吐  | TCP <1ms | IOPS 正常 | — |
| 网络延迟 | 延迟波动  | TCP connect↑ | IOPS 正常 | **L2** |
| 网络丢包 | 吞吐骤降  | TCP 重传/超时 | IOPS 正常 | **L2** |
| 磁盘繁忙 | 延迟升高吞吐降 | 网络正常 | await↑ iops↓ pressure_io↑ | **L3** |
| 服务端 OOM | 连接重置  | TCP 正常→应用超时 | mem_avail↓ | **L1+L3** |
| CPU 打满 | 延迟升高  | 网络正常 | cpu_iowait↑ | **L3** |

### 故障模拟

```bash
# 模拟 IO 压力
stress-ng --hdd 4 --hdd-bytes 4G --timeout 60s &
./stordiag doctor --layers system
# 观察: iops 飙升, await 变大, pressure_io 升高

# 模拟内存压力
stress-ng --vm 2 --vm-bytes 2G --timeout 60s &
./stordiag layers system
# 观察: memory_available 下降

# 模拟网络延迟 (需要 MinIO Docker)
docker exec minio tc qdisc add dev eth0 root netem delay 100ms
./stordiag layers network
# 观察: tcp_connect 延迟飙升到 ~100ms
```

## 配置方式

优先级：**CLI flags > 环境变量 > 配置文件 > 默认值**

### 环境变量

所有 CLI flags 均可通过 `STORDIAG_` 前缀环境变量设置：

```bash
export STORDIAG_ENDPOINT=localhost:9000
export STORDIAG_ACCESS_KEY=admin
export STORDIAG_SECRET_KEY=secret
export STORDIAG_BUCKET=data
export STORDIAG_SECURE=true
export STORDIAG_TIMEOUT=60
```

### 配置文件

支持 YAML 格式的 `.stordiag.yaml` 或 `.stordiag.json`，存放在当前目录或 `--config` 指定：

```yaml
endpoint: localhost:9000
access-key: admin
secret-key: secret
bucket: data
secure: false
```

## 与 eBPF 的分层配合

```
stordiag doctor --json              # ① 定期巡检: 发现"哪层有问题"
        ↓ 告警
stordiag layers system              # ② 聚焦特定层: 缩小范围
        ↓ 确认是 L3 磁盘问题
sudo biolatency                      # ③ eBPF 深挖: IO 延迟分布
sudo fileslower 10                   # ④ 文件级慢 IO 追踪
sudo trace block_rq_issue           # ⑤ 块层 IO 事件
```

| 工具 | 定位 | 部署 | 精度 | 场景 |
|---|---|---|---|---|
| stordiag | 业务视角端到端 | 零依赖, 普通用户 | ms/µs | 常态化巡检/CI |
| eBPF (bcc/bpftrace) | 内核视角逐层分解 | root, 特定内核 | ns | 深度排查/调优 |

stordiag 做 **第一道筛选**, eBPF 做 **按需深挖**。两者互补。

## 构建

```bash
make build                 # 当前平台
make cross                 # 交叉编译所有平台
make fmt                   # go fmt
make lint                  # go vet
make clean                 # 清理

# 单平台交叉编译
GOOS=linux GOARCH=arm64 go build -o stordiag-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -o stordiag-darwin-amd64 .
```

## License

MIT
