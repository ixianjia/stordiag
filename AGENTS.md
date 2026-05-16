# stordiag

## 项目概览

单二进制、零依赖的分布式存储诊断工具箱，支持 S3 兼容对象存储和 POSIX 文件系统三层分层诊断。

## 技术栈

- Go 1.22, single module: `github.com/chirs/stordiag`
- [cobra](https://github.com/spf13/cobra) CLI framework
- [viper](https://github.com/spf13/viper) config (CLI > env `STORDIAG_*` > `.stordiag.yaml` > defaults)
- [minio-go/v7](https://github.com/minio/minio-go) S3 driver

## 架构

```
main.go → cmd/{root,doctor,layers,health,bench,verify,version}.go
              ↓
internal/driver/{driver.go,posix.go,s3.go}   — Driver interface + 2 implementations
internal/probe/{probe.go,layer.go,network.go,system.go}  — 3-layer probe system
internal/report/report.go                    — text/JSON output
internal/checksum/checksum.go               — sha256/md5 verification
```

## 命令

| 命令 | 用途 |
|------|------|
| `doctor` | 全量三层诊断，`--layers all\|app\|network\|system` |
| `layers` | 单层独立执行 `layers app\|network\|system` |
| `health` | 健康检查 |
| `bench` | 性能压测 `bench read\|write`，`--size --concurrency --samples` |
| `verify` | 数据完整性校验 `verify <path> --algo sha256\|md5` |

所有命令支持 `--json` 输出。

## 端点自动检测

以 `/` 或 `.` 开头 → POSIX 本地路径，否则 → S3 `host:port`。

## 开发命令

```bash
make build       # go build -ldflags=... -o stordiag .
make lint        # go vet ./...
make fmt         # go fmt ./...
make cross       # 交叉编译 linux/darwin amd64/arm64
make clean       # rm -f stordiag
```

版本通过 `git describe --tags --always --dirty` 注入 ldflags。

## 测试

```bash
./test/run.sh                        # 一键运行所有集成测试场景
./test/scenario/01-disk-io.sh        # 单场景（需要 jq）
./test/scenario/02-network-lag.sh     # 需要 podman
```

没有单元测试（没有 `*_test.go` 文件）。所有测试都是集成测试，需要 podman、stress-ng、jq。

## 输出格式

- 文本：tabwriter 格式化表格
- JSON：`--json` 标志，indented encoding

## 风格

- Go 标准命名：导出函数 PascalCase，内部 snake_case
- 全局 driver 实例在 `cmd/root.go` 通过 `initDriver()` 创建
- 所有命令通过 cobra `PersistentPreRunE` 自动初始化 driver
- 默认值：endpoint=`localhost:9000`, AK/SK=`minioadmin`, bucket=`stordiag`
