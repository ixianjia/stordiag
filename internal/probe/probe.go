package probe

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chirs/stordiag/internal/driver"
)

type LatencySample struct {
	Op    string
	Size  int64
	Start time.Time
	Dur   time.Duration
	Err   error
}

type measurement struct {
	dur time.Duration
	op  string
	err error
}

type BenchConfig struct {
	Op          string        // "read" / "write" / "rw"
	Size        int64
	Concurrency int
	Samples     int           // 0 means use Duration
	Duration    time.Duration // 0 means use Samples
	RWMix       int           // 0–100, percentage of reads (only when Op="rw")
	Pattern     string        // "sequential" or "random"
	Warmup      int           // pre-write objects, 0 = auto
}

func MeasureLatency(ctx context.Context, drv driver.Driver, cfg BenchConfig) (driver.BenchResult, error) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Samples <= 0 && cfg.Duration <= 0 {
		cfg.Samples = 5
	}
	if cfg.RWMix < 0 {
		cfg.RWMix = 0
	}
	if cfg.RWMix > 100 {
		cfg.RWMix = 100
	}
	if cfg.Pattern == "" {
		cfg.Pattern = "sequential"
	}

	needsRead := cfg.Op == "read" || cfg.Op == "rw"
	warmupObjs := cfg.Warmup
	// For read-any workloads, ensure enough pre-written objects
	if needsRead && warmupObjs <= 0 {
		warmupObjs = cfg.Concurrency * 2
		if cfg.Samples > warmupObjs {
			warmupObjs = cfg.Samples
		}
	}

	var objPool []string
	if needsRead && warmupObjs > 0 {
		for i := 0; i < warmupObjs; i++ {
			path := benchPath(cfg.Pattern, i)
			buf := make([]byte, cfg.Size)
			if _, err := rand.Read(buf); err != nil {
				return driver.BenchResult{}, fmt.Errorf("warmup data: %w", err)
			}
			if err := drv.Write(ctx, path, bytes.NewReader(buf), cfg.Size); err != nil {
				return driver.BenchResult{}, fmt.Errorf("warmup write: %w", err)
			}
			objPool = append(objPool, path)
		}
		defer cleanupBenchObjects(ctx, drv)
	}

	var (
		measurements []measurement
		mu           sync.Mutex
		wg           sync.WaitGroup
		sem          = make(chan struct{}, cfg.Concurrency)
		workStart    time.Time
		workEnd      time.Time
		startOnce    sync.Once
		endMu        sync.Mutex
		opCount      int32
	)

	if cfg.Duration > 0 {
		// Duration mode: run fixed time, variable iterations
		workStart = time.Now()
		for i := 0; i < cfg.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					if time.Since(workStart) >= cfg.Duration {
						return
					}
					sem <- struct{}{}
					m := doOp(ctx, drv, cfg, objPool)
					<-sem
					mu.Lock()
					measurements = append(measurements, m)
					mu.Unlock()
					atomic.AddInt32(&opCount, 1)
				}
			}()
		}
		wg.Wait()
		workEnd = time.Now()
	} else {
		// Samples mode: fixed iterations
		total := cfg.Samples
		if cfg.Op == "rw" {
			total = cfg.Samples
		}
		for i := 0; i < total; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				startOnce.Do(func() { workStart = time.Now() })

				m := doOp(ctx, drv, cfg, objPool)

				endMu.Lock()
				workEnd = time.Now()
				endMu.Unlock()
				mu.Lock()
				measurements = append(measurements, m)
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		if workStart.IsZero() {
			workStart = time.Now()
		}
		if workEnd.IsZero() {
			workEnd = time.Now()
		}
	}

	cleanupBenchObjects(ctx, drv)

	if len(measurements) == 0 {
		return driver.BenchResult{}, fmt.Errorf("no measurements collected")
	}

	effectiveDur := workEnd.Sub(workStart)
	if effectiveDur <= 0 {
		effectiveDur = time.Nanosecond
	}

	var durs []float64
	var errCount int
	var totalBytes int64
	op := cfg.Op
	for _, m := range measurements {
		if m.err != nil {
			errCount++
			continue
		}
		durs = append(durs, float64(m.dur.Nanoseconds()))
		totalBytes += cfg.Size
		if op == "rw" {
			op = m.op
		}
	}

	if len(durs) == 0 {
		return driver.BenchResult{}, fmt.Errorf("all %d measurements failed", len(measurements))
	}

	sort.Float64s(durs)

	p50 := percentile(durs, 50)
	p95 := percentile(durs, 95)
	p99 := percentile(durs, 99)
	p999 := percentile(durs, 99.9)

	throughput := float64(totalBytes) / (1024 * 1024) / effectiveDur.Seconds()
	iops := float64(len(durs)) / effectiveDur.Seconds()

	tier := driver.TierThroughput
	if cfg.Size < 65536 && iops > 1000 {
		tier = driver.TierIOPS
	}

	return driver.BenchResult{
		Op:          op,
		Size:        cfg.Size,
		Concurrency: cfg.Concurrency,
		Duration:    effectiveDur,
		Throughput:  throughput,
		IOPS:        iops,
		Tier:        tier,
		AvgLatency:  time.Duration(int64(average(durs))),
		P50Latency:  time.Duration(p50),
		P95Latency:  time.Duration(p95),
		P99Latency:  time.Duration(p99),
		P999Latency: time.Duration(p999),
		MinLatency:  time.Duration(durs[0]),
		MaxLatency:  time.Duration(durs[len(durs)-1]),
		Errors:      errCount,
	}, nil
}

func doOp(ctx context.Context, drv driver.Driver, cfg BenchConfig, objPool []string) measurement {
	isRead := cfg.Op == "read"
	if cfg.Op == "rw" {
		r := cfg.RWMix
		if r <= 0 {
			isRead = false
		} else if r >= 100 {
			isRead = true
		} else {
			buf := make([]byte, 1)
			rand.Read(buf)
			isRead = int(buf[0]) < (r*256/100)
		}
	}

	path := benchPath(cfg.Pattern, int(time.Now().UnixNano()))

	if isRead && len(objPool) > 0 {
		path = objPool[time.Now().UnixNano()%int64(len(objPool))]
	}

	if isRead {
		dur, err := doRead(ctx, drv, path, cfg.Size)
		return measurement{dur: dur, op: "read", err: err}
	}
	dur, err := doWrite(ctx, drv, path, cfg.Size)
	return measurement{dur: dur, op: "write", err: err}
}

func doRead(ctx context.Context, drv driver.Driver, path string, size int64) (time.Duration, error) {
	start := time.Now()
	err := drv.Read(ctx, path, io.Discard)
	return time.Since(start), err
}

func doWrite(ctx context.Context, drv driver.Driver, path string, size int64) (time.Duration, error) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.CopyN(pw, zeroReader(size), size)
		pw.Close()
	}()
	start := time.Now()
	err := drv.Write(ctx, path, pr, size)
	return time.Since(start), err
}

func benchPath(pattern string, idx int) string {
	return fmt.Sprintf("stordiag_bench_%d", idx)
}

func cleanupBenchObjects(ctx context.Context, drv driver.Driver) {
	entries, err := drv.List(ctx, "stordiag_bench_")
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = drv.Delete(ctx, e.Name)
	}
}

type zeroReader int64

func (z zeroReader) Read(p []byte) (int, error) {
	n := len(p)
	if int64(n) > int64(z) {
		n = int(z)
	}
	for i := range p[:n] {
		p[i] = 0
	}
	return n, nil
}

func percentile(sorted []float64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*p/100)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return int64(sorted[idx])
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
