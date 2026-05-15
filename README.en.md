# stordiag — Storage Diagnostics Toolkit

> **Single binary, zero-dependency** distributed storage diagnostics.
> Supports **S3-compatible object storage** and **POSIX filesystem** with three-layer hierarchical diagnosis.

## Architecture

| Layer | Name | Probes | Data Source |
|---|---|---|---|
| **L1** | Application | ping / bench (read/write) / data integrity | Storage API end-to-end |
| **L2** | Network | DNS lookup / TCP connect / TLS handshake | `net` + `crypto/tls` |
| **L3** | System | disk IOPS+await / memory / CPU iowait / Pressure Stall | `/proc/*` |

Diagnostics aggregated by layer:

```
  Application  OK=3 WARN=0 FAIL=0
  Network      OK=2 WARN=0 FAIL=0
  System       OK=5 WARN=0 FAIL=0
```

## Quick Start

### Build

```bash
go build -o stordiag .
# or
make build
```

### POSIX (Local Filesystem)

```bash
# Full three-layer diagnosis
./stordiag doctor --endpoint /tmp/stordiag_test

# System layer only
./stordiag doctor --endpoint /tmp/stordiag_test --layers system

# Single layer
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

## Command Reference

### Global Flags

| Flag | Env | Default | Description |
|---|---|---|---|
| `--endpoint` | `STORDIAG_ENDPOINT` | `localhost:9000` | Storage endpoint. POSIX: path, S3: `host:port` |
| `--access-key` | `STORDIAG_ACCESS_KEY` | `minioadmin` | S3 access key. Ignored for POSIX. |
| `--secret-key` | `STORDIAG_SECRET_KEY` | `minioadmin` | S3 secret key. Ignored for POSIX. |
| `--bucket` | `STORDIAG_BUCKET` | `stordiag` | S3 bucket name. Ignored for POSIX. |
| `--secure` | `STORDIAG_SECURE` | `false` | Enable TLS for S3 |
| `--timeout` | `STORDIAG_TIMEOUT` | `30` | Operation timeout (seconds) |
| `--json` | — | `false` | JSON output |
| `--config` | — | `.stordiag.yaml` | Config file path |

Auto-detection: endpoint starting with `/` or `.` → POSIX path, otherwise → S3 `host:port`.

All flags can also be set via `STORDIAG_` env vars or a YAML/JSON config file. Priority: **CLI > Env > Config > Default**.

---

### `doctor` — Full Three-Layer Diagnosis

```
stordiag doctor [flags]
```

**Flags**: `--layers` (default `all`) — `all` / `app` / `network` / `system`

Probes:

| Layer | Probe | Measurement |
|---|---|---|
| L1:app | `ping` | Connectivity + RTT |
| L1:app | `bench_write` | 4MiB write throughput + p99 latency |
| L1:app | `data_integrity` | 512KiB end-to-end write→read→sha256 |
| L2:network | `dns_lookup` | DNS resolution latency |
| L2:network | `tcp_connect` | TCP connect latency |
| L2:network | `tls_handshake` | TLS handshake latency + protocol version |
| L3:system | `disk_*` | Block device IOPS / await / queue depth |
| L3:system | `memory_available` | Available memory (MiB) |
| L3:system | `pressure_cpu` | CPU Pressure Stall (avg60) |
| L3:system | `pressure_io` | IO Pressure Stall (avg60) |
| L3:system | `cpu_iowait` | /proc/stat iowait ticks |

```bash
# Full three-layer
stordiag doctor --endpoint /data

# Application layer only
stordiag doctor --layers app --endpoint /data

# CI integration
stordiag doctor --json --endpoint /data | jq '.Summary'
```

Output example:

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

### `layers` — Single Layer Execution

```
stordiag layers <app|network|system> [flags]
```

```bash
# System layer only
stordiag layers system --endpoint /data

# Network layer JSON output
stordiag layers network --endpoint s3.example.com:443 --json

# Application layer (quick check)
stordiag layers app --endpoint minio-cluster:9000
```

---

### `health` — Health Check

```
stordiag health [target] [flags]
```

Quickly check connectivity and basic status. Probes: `root_stat`, `list_objects`.

```bash
stordiag health --endpoint /mnt/data
stordiag health --endpoint 10.0.0.1:9000
stordiag health --json --endpoint /data
```

---

### `bench` — Performance Benchmark

```
stordiag bench <read|write> [flags]
```

**Flags**: `--size` (default 4MiB), `--concurrency` (default 4), `--samples` (default 10)

| Scenario | size | concurrency | samples |
|---|---|---|---|
| Throughput baseline | 4MiB | 4-8 | 10-30 |
| IOPS baseline | 4KiB | 16-64 | 100-500 |
| Large sequential | 64MiB | 1-2 | 5-10 |
| Extreme stress | 1MiB | 64-128 | 50 |

```bash
stordiag bench write --size=4M --concurrency=4
stordiag bench write --size=64K --concurrency=8 --samples=100
stordiag bench read --size=64M --concurrency=2 --samples=5
stordiag bench write --size=4M --concurrency=16 --endpoint minio:9000
```

---

### `verify` — Data Integrity

End-to-end verification: write → read → SHA256/MD5 comparison. Detects silent data corruption and bit rot.

```
stordiag verify <path> [flags]
```

**Flags**: `--algo` (default `sha256`, also supports `md5`)

```bash
stordiag verify /backup/db.sql --endpoint /data
stordiag verify test.dat --algo md5
stordiag verify important.bin --endpoint s3.company.com
```

## Fault Diagnosis Matrix

| Scenario | L1 Application | L2 Network | L3 System | Root Cause |
|---|---|---|---|---|
| Normal | Low latency, high throughput | TCP <1ms | Normal IOPS | — |
| Network latency | Latency spikes | TCP connect↑ | Normal IOPS | **L2** |
| Packet loss | Throughput drops | TCP retransmit/timeout | Normal IOPS | **L2** |
| Busy disk | Latency ↑, throughput ↓ | Network normal | await↑ iops↓ | **L3** |
| Server OOM | Connection reset | TCP normal → app timeout | mem_avail↓ | **L1+L3** |
| CPU saturation | Latency ↑ | Network normal | cpu_iowait↑ | **L3** |

## License

MIT
