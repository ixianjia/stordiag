package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

type Format int

const (
	FormatText Format = iota
	FormatJSON
)

type HealthReport struct {
	Target    string
	Type      string
	Reachable bool
	Latency   time.Duration
	Error     string
	Checks    []CheckItem
	Timestamp time.Time
}

type CheckItem struct {
	Name   string
	Status string // OK / WARN / FAIL
	Value  string
	Detail string
}

type BenchReport struct {
	Target      string
	Type        string
	Op          string
	Size        string
	Concurrency int
	Duration    time.Duration
	Throughput  float64
	AvgLatency  time.Duration
	P99Latency  time.Duration
	Errors      int
}

type VerifyReport struct {
	Target     string
	Type       string
	Path       string
	Algo       string
	RemoteHash string
	LocalHash  string
	Match      bool
	Timestamp  time.Time
}

type LayerProbe struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Value   string `json:"value,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type LayerReportItem struct {
	Layer   string       `json:"layer"`
	Label   string       `json:"label"`
	OK      int          `json:"ok"`
	WARN    int          `json:"warn"`
	FAIL    int          `json:"fail"`
	Probes  []LayerProbe `json:"probes"`
}

type DoctorReport struct {
	Target    string
	Type      string
	Layers    []LayerReportItem
	Summary   string
	Timestamp time.Time
}

func PrintHealth(w io.Writer, r HealthReport, format Format) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
	default:
		fmt.Fprintf(w, "=== Health Check: %s (%s) ===\n", r.Target, r.Type)
		fmt.Fprintf(w, "  Timestamp:  %s\n", r.Timestamp.Format(time.RFC3339))
		fmt.Fprintf(w, "  Reachable:  %v\n", r.Reachable)
		fmt.Fprintf(w, "  Latency:    %s\n", r.Latency)
		if r.Error != "" {
			fmt.Fprintf(w, "  Error:      %s\n", r.Error)
		}
		if len(r.Checks) > 0 {
			fmt.Fprintln(w)
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "Check\tStatus\tValue\tDetail")
			fmt.Fprintln(tw, "-----\t------\t-----\t------")
			for _, c := range r.Checks {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Name, c.Status, c.Value, c.Detail)
			}
			tw.Flush()
		}
	}
}

func PrintBench(w io.Writer, r BenchReport, format Format) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
	default:
		fmt.Fprintf(w, "=== Bench: %s (%s) ===\n", r.Target, r.Type)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "Metric\tValue")
		fmt.Fprintln(tw, "------\t-----")
		fmt.Fprintf(tw, "Operation\t%s\n", r.Op)
		fmt.Fprintf(tw, "Size\t%s\n", r.Size)
		fmt.Fprintf(tw, "Concurrency\t%d\n", r.Concurrency)
		fmt.Fprintf(tw, "Duration\t%s\n", r.Duration.String())
		fmt.Fprintf(tw, "Throughput\t%.2f MB/s\n", r.Throughput)
		fmt.Fprintf(tw, "Avg Latency\t%s\n", r.AvgLatency.String())
		fmt.Fprintf(tw, "P99 Latency\t%s\n", r.P99Latency.String())
		fmt.Fprintf(tw, "Errors\t%d\n", r.Errors)
		tw.Flush()
	}
}

func PrintVerify(w io.Writer, r VerifyReport, format Format) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
	default:
		fmt.Fprintf(w, "=== Verify: %s/%s ===\n", r.Target, r.Path)
		fmt.Fprintf(w, "  Algorithm:  %s\n", r.Algo)
		fmt.Fprintf(w, "  Remote:     %s\n", r.RemoteHash)
		fmt.Fprintf(w, "  Local:      %s\n", r.LocalHash)
		if r.Match {
			fmt.Fprintf(w, "  Result:     OK (checksums match)\n")
		} else {
			fmt.Fprintf(w, "  Result:     FAIL (checksums mismatch!)\n")
		}
	}
}

func PrintDoctor(w io.Writer, r DoctorReport, format Format) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
	default:
		fmt.Fprintf(w, "=== Doctor Report: %s (%s) ===\n", r.Target, r.Type)
		fmt.Fprintf(w, "  Timestamp:  %s\n", r.Timestamp.Format(time.RFC3339))
		fmt.Fprintf(w, "  Summary:    %s\n", r.Summary)
		for _, lr := range r.Layers {
			printLayerSection(w, lr)
		}
	}
}

func PrintLayerOutput(w io.Writer, lr LayerReportItem, format Format) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(lr)
	default:
		printLayerSection(w, lr)
	}
}

func printLayerSection(w io.Writer, lr LayerReportItem) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "\n--- %s [%s]  OK=%d  WARN=%d  FAIL=%d ---\n",
		lr.Label, lr.Layer, lr.OK, lr.WARN, lr.FAIL)
	fmt.Fprintln(tw, "Probe\tStatus\tLatency\tValue\tDetail")
	fmt.Fprintln(tw, "-----\t------\t-------\t-----\t------")
	for _, p := range lr.Probes {
		lat := p.Latency
		if lat == "0s" {
			lat = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Name, p.Status, lat, p.Value, p.Detail)
	}
	tw.Flush()
}

func FormatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

var Stderr = os.Stderr

func PrintTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Repeat("---\t", len(headers)))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}
