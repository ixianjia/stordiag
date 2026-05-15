package probe

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"
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

func MeasureLatency(ctx context.Context, drv driver.Driver, op string, size int64, concurrency int, samples int) (driver.BenchResult, error) {
	if concurrency <= 0 {
		concurrency = 1
	}
	if samples <= 0 {
		samples = 5
	}

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return driver.BenchResult{}, fmt.Errorf("generate data: %w", err)
	}

	type result struct {
		dur time.Duration
		err error
	}

	var (
		results []result
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, concurrency)
		start   = time.Now()
	)

	for i := 0; i < samples; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			path := fmt.Sprintf("stordiag_bench_%d_%d", time.Now().UnixNano(), i)
			r := result{}
			if op == "write" {
				r.dur, r.err = measureWrite(ctx, drv, path, buf)
			} else {
				r.dur, r.err = measureRead(ctx, drv, path, buf)
			}

			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	total := time.Since(start)

	if len(results) == 0 {
		return driver.BenchResult{}, fmt.Errorf("no samples collected")
	}

	// cleanup bench objects
	cleanupBenchObjects(ctx, drv)

	var durs []float64
	var errCount int
	var totalDur time.Duration
	for _, r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		durs = append(durs, float64(r.dur.Nanoseconds()))
		totalDur += r.dur
	}

	if len(durs) == 0 {
		return driver.BenchResult{}, fmt.Errorf("all samples errored")
	}

	sort.Float64s(durs)
	avgNs := totalDur.Nanoseconds() / int64(len(durs))
	p99Idx := int(math.Ceil(float64(len(durs))*0.99)) - 1
	if p99Idx < 0 {
		p99Idx = 0
	}
	if p99Idx >= len(durs) {
		p99Idx = len(durs) - 1
	}

	throughput := float64(size*int64(samples)) / (1024 * 1024) / total.Seconds()

	return driver.BenchResult{
		Op:          op,
		Size:        size,
		Concurrency: concurrency,
		Duration:    total,
		Throughput:  throughput,
		AvgLatency:  time.Duration(avgNs),
		P99Latency:  time.Duration(durs[p99Idx]),
		Errors:      errCount,
	}, nil
}

func measureWrite(ctx context.Context, drv driver.Driver, path string, buf []byte) (time.Duration, error) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write(buf)
		pw.Close()
	}()
	start := time.Now()
	err := drv.Write(ctx, path, pr, int64(len(buf)))
	return time.Since(start), err
}

func measureRead(ctx context.Context, drv driver.Driver, path string, buf []byte) (time.Duration, error) {
	// write first, then measure read
	if err := drv.Write(ctx, path, nilReader(len(buf)), int64(len(buf))); err != nil {
		return 0, fmt.Errorf("setup write: %w", err)
	}
	start := time.Now()
	err := drv.Read(ctx, path, io.Discard)
	return time.Since(start), err
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

type nilReader int64

func (n nilReader) Read(p []byte) (int, error) {
	if n <= 0 {
		return 0, io.EOF
	}
	for i := range p {
		p[i] = 0
	}
	if int64(len(p)) > int64(n) {
		return int(n), io.EOF
	}
	return len(p), nil
}

func (n nilReader) WriteTo(w io.Writer) (int64, error) {
	return io.CopyN(w, n, int64(n))
}
