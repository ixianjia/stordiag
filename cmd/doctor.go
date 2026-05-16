package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/checksum"
	"github.com/chirs/stordiag/internal/driver"
	"github.com/chirs/stordiag/internal/probe"
	"github.com/chirs/stordiag/internal/report"
)

var doctorLayers string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Comprehensive diagnostics across all layers",
	Long: `Run diagnostics across all three layers:
  L1 (app)     — health, bench, data integrity
  L2 (network) — DNS, TCP, TLS breakdown
  L3 (system)  — disk IO, memory, CPU iowait, pressure stall`,
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized")
		}

		fmt.Fprintf(os.Stderr, "=== Running diagnostics on %s ===\n\n", drv)

		ctx, cancel := ctx()
		defer cancel()

		dr := report.DoctorReport{
			Target:    drv.String(),
			Type:      drv.Type(),
			Timestamp: time.Now(),
		}

		// === L1: Application Layer ===
		l1 := runAppLayer(ctx, drv)
		dr.Layers = append(dr.Layers, toLayerReportItem("L1:app", "Application", l1))

		// === L2: Network Layer ===
		if layerEnabled("network") {
			l2 := probe.ProbeNetwork(ctx, drv)
			dr.Layers = append(dr.Layers, toLayerReportItem("L2:network", "Network", l2))
		}

		// === L3: System Layer ===
		if layerEnabled("system") {
			l3 := probe.ProbeSystem(ctx, drv)
			dr.Layers = append(dr.Layers, toLayerReportItem("L3:system", "System", l3))
		}

		// Summary
		dr.Summary = summarizeLayers(dr.Layers)

		f := report.FormatText
		if jsonOut {
			f = report.FormatJSON
		}
		// Print layer summaries at the top
		for _, l := range dr.Layers {
			fmt.Fprintf(os.Stderr, "  %s  OK=%d WARN=%d FAIL=%d\n", l.Label, l.OK, l.WARN, l.FAIL)
		}
		fmt.Fprintln(os.Stderr)

		report.PrintDoctor(os.Stdout, dr, f)
		return nil
	},
}

func runAppLayer(ctx context.Context, drv driver.Driver) *probe.LayerReport {
	r := &probe.LayerReport{Layer: probe.LayerApp}

	// 1. Health check
	hr := drv.Ping(ctx)
	if !hr.Reachable {
		r.Add("ping", "FAIL", hr.Latency, "unreachable", hr.Error)
		return r
	}
	r.Add("ping", "OK", hr.Latency, "reachable", "")

	// 2. Quick bench
	br, err := probe.MeasureLatency(ctx, drv, probe.BenchConfig{
		Op:          "write",
		Size:        4 * 1024 * 1024,
		Concurrency: 1,
		Samples:     3,
	})
	if err != nil {
		r.Add("bench_write", "WARN", 0, err.Error(), "")
	} else {
		r.Add("bench_write", "OK", br.AvgLatency,
			fmt.Sprintf("%.1f MB/s  p99=%s", br.Throughput, br.P99Latency), "")
	}

	// 3. Data integrity
	testPath := fmt.Sprintf("stordiag_doctor_vrfy_%d", time.Now().UnixNano())
	data := make([]byte, 512*1024)
	_, _ = rand.Read(data)
	v := &checksum.Verifier{Driver: drv}
	if ok, err := v.VerifyEndToEnd(ctx, testPath, checksum.SHA256, data); err != nil {
		r.Add("data_integrity", "FAIL", 0, err.Error(), "")
	} else if ok {
		r.Add("data_integrity", "OK", 0, "checksums match", "sha256")
	} else {
		r.Add("data_integrity", "FAIL", 0, "checksums mismatch!", "possible silent corruption")
	}

	// cleanup verify object
	_ = drv.Delete(ctx, testPath)

	return r
}

func toLayerReportItem(id, label string, lr *probe.LayerReport) report.LayerReportItem {
	ok, warn, fail := lr.Summary()
	probes := make([]report.LayerProbe, len(lr.Results))
	for i, p := range lr.Results {
		probes[i] = report.LayerProbe{
			Name:   p.Name,
			Status: p.Status,
			Latency: durationStr(p.Latency),
			Value:  p.Value,
			Detail: p.Detail,
		}
	}
	return report.LayerReportItem{
		Layer:  id,
		Label:  label,
		OK:     ok,
		WARN:   warn,
		FAIL:   fail,
		Probes: probes,
	}
}

func durationStr(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return d.String()
}

func layerEnabled(name string) bool {
	return doctorLayers == "all" || doctorLayers == name
}

func summarizeLayers(layers []report.LayerReportItem) string {
	var totalFail, totalWarn int
	for _, l := range layers {
		totalFail += l.FAIL
		totalWarn += l.WARN
	}
	switch {
	case totalFail > 0:
		return fmt.Sprintf("FAIL (%d failures)", totalFail)
	case totalWarn > 0:
		return fmt.Sprintf("PASS with %d warnings", totalWarn)
	default:
		return "PASS"
	}
}

func init() {
	doctorCmd.Flags().StringVar(&doctorLayers, "layers", "all",
		"Layers to probe: all, app, network, system")
	rootCmd.AddCommand(doctorCmd)
}
