package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/probe"
	"github.com/chirs/stordiag/internal/report"
)

var (
	benchSize    int
	benchConc    int
	benchSamples int
	benchDur    string
	benchRWMix  int
	benchWarmup int
)

var benchCmd = &cobra.Command{
	Use:   "bench [read|write|rw]",
	Short: "Run performance benchmark (read/write/mixed)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized")
		}

		op := args[0]
		switch op {
		case "read", "write", "rw":
		default:
			return fmt.Errorf("op must be 'read', 'write', or 'rw', got %q", op)
		}

		var duration time.Duration
		if benchDur != "" {
			var err error
			duration, err = time.ParseDuration(benchDur)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", benchDur, err)
			}
		}

		ctx, cancel := ctx()
		defer cancel()

		cfg := probe.BenchConfig{
			Op:          op,
			Size:        int64(benchSize),
			Concurrency: benchConc,
			Samples:     benchSamples,
			Duration:    duration,
			RWMix:       benchRWMix,
			Warmup:      benchWarmup,
		}

		result, err := probe.MeasureLatency(ctx, drv, cfg)
		if err != nil {
			return fmt.Errorf("bench %s: %w", op, err)
		}

		f := report.FormatText
		if jsonOut {
			f = report.FormatJSON
		}
		report.PrintBench(os.Stdout, report.BenchReport{
			Target:      drv.String(),
			Type:        drv.Type(),
			Op:          result.Op,
			Size:        report.FormatBytes(result.Size),
			Concurrency: result.Concurrency,
			Duration:    result.Duration,
			Throughput:  result.Throughput,
			IOPS:        result.IOPS,
			Tier:        string(result.Tier),
			AvgLatency:  result.AvgLatency,
			P50Latency:  result.P50Latency,
			P95Latency:  result.P95Latency,
			P99Latency:  result.P99Latency,
			P999Latency: result.P999Latency,
			MinLatency:  result.MinLatency,
			MaxLatency:  result.MaxLatency,
			Errors:      result.Errors,
		}, f)
		return nil
	},
}

func init() {
	benchCmd.Flags().IntVar(&benchSize, "size", 4*1024*1024, "Data size per operation (bytes)")
	benchCmd.Flags().IntVar(&benchConc, "concurrency", 4, "Number of concurrent operations")
	benchCmd.Flags().IntVar(&benchSamples, "samples", 10, "Number of samples (ignored if --duration set)")
	benchCmd.Flags().StringVar(&benchDur, "duration", "", "Run for duration (e.g. 60s, 5m) instead of fixed samples")
	benchCmd.Flags().IntVar(&benchRWMix, "rw-mix", 50, "Read percentage for rw mode (0–100)")
	benchCmd.Flags().IntVar(&benchWarmup, "warmup", 0, "Pre-write objects for read benchmark (0=auto)")
	rootCmd.AddCommand(benchCmd)
}
