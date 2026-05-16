package driver

import (
	"context"
	"io"
	"time"
)

type Stat struct {
	Name  string
	Size  int64
	IsDir bool
	Mode  string
	ETag  string
	Mtime time.Time
}

type HealthResult struct {
	Reachable bool
	Latency   time.Duration
	Error     string
}

type Tier string

const (
	TierThroughput Tier = "throughput"
	TierIOPS       Tier = "iops"
)

type BenchResult struct {
	Op          string        // read / write
	Size        int64
	Concurrency int
	Duration    time.Duration
	Throughput  float64 // MB/s
	IOPS        float64
	Tier        Tier
	AvgLatency  time.Duration
	P50Latency  time.Duration
	P95Latency  time.Duration
	P99Latency  time.Duration
	P999Latency time.Duration
	MinLatency  time.Duration
	MaxLatency  time.Duration
	Errors      int
}

type Driver interface {
	Type() string
	String() string
	Ping(ctx context.Context) HealthResult
	Stat(ctx context.Context, path string) (Stat, error)
	List(ctx context.Context, prefix string) ([]Stat, error)
	Read(ctx context.Context, path string, w io.Writer) error
	Write(ctx context.Context, path string, r io.Reader, size int64) error
	Delete(ctx context.Context, path string) error
}
