# stordiag 生产场景复现测试套件

## 前置依赖

```bash
podman         # 场景 2 和 4 需要
stress-ng      # 场景 1 推荐 (可选, 无则用 dd 模拟)
jq             # 诊断输出解析
```

## 使用方法

```bash
# 一键运行所有场景
./test/run.sh

# 单场景执行
./test/scenario/01-disk-io.sh       # POSIX 本地
./test/scenario/03-data-corrupt.sh   # POSIX 本地
./test/scenario/02-network-lag.sh    # 需要 podman
./test/scenario/04-service-down.sh   # 需要 podman

# 指定输出目录
./test/run.sh /tmp/my_report
```

## 场景说明

| # | 场景 | 依赖 | 耗时 | 预期 |
|---|------|------|------|------|
| 1 | 磁盘 IO 打满 | stress-ng (推荐) | ~75s | L3 await↑ |
| 2 | 网络高延迟 + 丢包 | podman | ~30s | L2 tcp_connect↑ |
| 3 | 静默数据损坏 | 无 | ~15s | L1 checksum FAIL |
| 4 | 服务不可用 | podman | ~20s | L1+L2 FAIL |

## 诊断报告

每场景生成两份文件:

```
/tmp/stordiag_scenarios/
├── 01-disk-io_baseline.json      # 压力前基线
├── 01-disk-io_stress.json        # 压力下诊断
├── 01-disk-io_meta.json          # 关键指标摘要
├── 02-network-lag_*.json         # 网络场景
├── 03-data-corrupt_*.json        # 数据损坏场景
├── 04-service-down_*.json        # 服务中断场景
└── summary.json                  # 聚合报告
```
