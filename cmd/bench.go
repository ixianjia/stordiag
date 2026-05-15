package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/probe"
	"github.com/chirs/stordiag/internal/report"
)

var (
	benchSize    int
	benchConc    int
	benchSamples int
)

var benchCmd = &cobra.Command{
	Use:   "bench [read|write]",
	Short: "Run performance benchmark (read/write)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized")
		}

		op := args[0]
		if op != "read" && op != "write" {
			return fmt.Errorf("op must be 'read' or 'write', got %q", op)
		}

		ctx, cancel := ctx()
		defer cancel()

		result, err := probe.MeasureLatency(ctx, drv, op, int64(benchSize), benchConc, benchSamples)
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
			AvgLatency:  result.AvgLatency,
			P99Latency:  result.P99Latency,
			Errors:      result.Errors,
		}, f)
		return nil
	},
}

func init() {
	benchCmd.Flags().IntVar(&benchSize, "size", 4*1024*1024, "Data size per operation (bytes)")
	benchCmd.Flags().IntVar(&benchConc, "concurrency", 4, "Number of concurrent operations")
	benchCmd.Flags().IntVar(&benchSamples, "samples", 10, "Number of samples")
	rootCmd.AddCommand(benchCmd)
}
