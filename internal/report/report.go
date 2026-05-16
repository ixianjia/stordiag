package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

type Format int

const (
	FormatText Format = iota
	FormatJSON
	FormatPrometheus
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
	IOPS        float64
	Tier        string
	AvgLatency  time.Duration
	P50Latency  time.Duration
	P95Latency  time.Duration
	P99Latency  time.Duration
	P999Latency time.Duration
	MinLatency  time.Duration
	MaxLatency  time.Duration
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
		fmt.Fprintf(tw, "Tier\t%s\n", r.Tier)
		fmt.Fprintf(tw, "Size\t%s\n", r.Size)
		fmt.Fprintf(tw, "Concurrency\t%d\n", r.Concurrency)
		fmt.Fprintf(tw, "Duration\t%s\n", r.Duration.String())
		fmt.Fprintf(tw, "Throughput\t%.2f MB/s\n", r.Throughput)
		fmt.Fprintf(tw, "IOPS\t%.0f\n", r.IOPS)
		fmt.Fprintf(tw, "Min Latency\t%s\n", r.MinLatency.String())
		fmt.Fprintf(tw, "Avg Latency\t%s\n", r.AvgLatency.String())
		fmt.Fprintf(tw, "P50 Latency\t%s\n", r.P50Latency.String())
		fmt.Fprintf(tw, "P95 Latency\t%s\n", r.P95Latency.String())
		fmt.Fprintf(tw, "P99 Latency\t%s\n", r.P99Latency.String())
		fmt.Fprintf(tw, "P99.9 Latency\t%s\n", r.P999Latency.String())
		fmt.Fprintf(tw, "Max Latency\t%s\n", r.MaxLatency.String())
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

func PrintDoctorPrometheus(w io.Writer, r DoctorReport) {
	ts := r.Timestamp.Unix()
	labels := fmt.Sprintf(`endpoint="%s",type="%s"`, r.Target, r.Type)

	fmt.Fprintf(w, "# HELP stordiag_summary Diagnosis summary (0=PASS 1=WARN 2=FAIL)\n")
	fmt.Fprintf(w, "# TYPE stordiag_summary gauge\n")
	summaryVal := 0
	if len(r.Summary) >= 4 && r.Summary[:4] == "FAIL" {
		summaryVal = 2
	} else if len(r.Summary) >= 4 && r.Summary[:4] == "PASS" && len(r.Summary) > 4 {
		summaryVal = 1
	}
	fmt.Fprintf(w, "stordiag_summary{%s} %d %d\n", labels, summaryVal, ts)

	fmt.Fprintf(w, "\n# HELP stordiag_layer_probes Layer probe counts\n")
	fmt.Fprintf(w, "# TYPE stordiag_layer_probes gauge\n")
	for _, l := range r.Layers {
		ll := fmt.Sprintf(`layer="%s",label="%s",%s`, l.Layer, l.Label, labels)
		fmt.Fprintf(w, "stordiag_layer_ok{%s} %d %d\n", ll, l.OK, ts)
		fmt.Fprintf(w, "stordiag_layer_warn{%s} %d %d\n", ll, l.WARN, ts)
		fmt.Fprintf(w, "stordiag_layer_fail{%s} %d %d\n", ll, l.FAIL, ts)
	}

	fmt.Fprintf(w, "\n# HELP stordiag_probe_status Probe status (0=OK 1=WARN 2=FAIL 3=N/A)\n")
	fmt.Fprintf(w, "# TYPE stordiag_probe_status gauge\n")
	for _, l := range r.Layers {
		for _, p := range l.Probes {
			statusVal := 0
			switch p.Status {
			case "OK":
				statusVal = 0
			case "WARN":
				statusVal = 1
			case "FAIL":
				statusVal = 2
			default:
				statusVal = 3
			}
			pl := fmt.Sprintf(`layer="%s",probe="%s",%s`, l.Layer, p.Name, labels)
			fmt.Fprintf(w, "stordiag_probe_status{%s} %d %d\n", pl, statusVal, ts)
		}
	}
}

func PrintDoctorHTML(w io.Writer, r DoctorReport) {
	t := r.Timestamp.Format("2006-01-02 15:04:05")

	fmt.Fprint(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>stordiag Report</title>
<style>
body{font-family:system-ui,sans-serif;max-width:960px;margin:40px auto;padding:0 20px;color:#333}
h1{font-size:1.4rem;border-bottom:2px solid #eee;padding-bottom:8px}
table{width:100%;border-collapse:collapse;margin:12px 0 24px}
th,td{text-align:left;padding:8px 12px;border-bottom:1px solid #eee}
th{background:#f8f9fa;font-weight:600}
.summary{padding:8px 16px;border-radius:6px;display:inline-block;font-weight:600}
.PASS{background:#d4edda;color:#155724}
.WARN{background:#fff3cd;color:#856404}
.FAIL{background:#f8d7da;color:#721c24}
.probe-OK{color:#155724}
.probe-WARN{color:#856404}
.probe-FAIL,.probe-unknown{color:#721c24}
.probe-NA{color:#6c757d}
.meta{color:#666;font-size:0.9rem}
</style></head><body>
<h1>stordiag Diagnostic Report</h1>
<p class="meta">Target: `+htmlEsc(r.Target)+` (`+htmlEsc(r.Type)+`)<br>Timestamp: `+t+`</p>
<p>Summary: <span class="summary `+htmlEsc(r.Summary)+`">`+htmlEsc(r.Summary)+`</span></p>`)

	for _, l := range r.Layers {
		fmt.Fprintf(w, `<h2>%s <span class="meta">[%s]</span></h2>`, htmlEsc(l.Label), htmlEsc(l.Layer))
		fmt.Fprintf(w, `<p>OK=%d WARN=%d FAIL=%d</p>`, l.OK, l.WARN, l.FAIL)
		if len(l.Probes) > 0 {
			fmt.Fprint(w, `<table><tr><th>Probe</th><th>Status</th><th>Value</th><th>Detail</th></tr>`)
			for _, p := range l.Probes {
				statusClass := "probe-" + probeStatusClass(p.Status)
				fmt.Fprintf(w, `<tr><td>%s</td><td class="%s">%s</td><td>%s</td><td>%s</td></tr>`,
					htmlEsc(p.Name), statusClass, htmlEsc(p.Status), htmlEsc(p.Value), htmlEsc(p.Detail))
			}
			fmt.Fprint(w, `</table>`)
		}
	}
	fmt.Fprint(w, `</body></html>`)
}

func probeStatusClass(s string) string {
	switch s {
	case "OK":
		return "OK"
	case "WARN":
		return "WARN"
	case "FAIL":
		return "FAIL"
	case "N/A":
		return "NA"
	default:
		return "unknown"
	}
}

func htmlEsc(s string) string {
	var out []byte
	for _, c := range []byte(s) {
		switch c {
		case '&':
			out = append(out, "&amp;"...)
		case '<':
			out = append(out, "&lt;"...)
		case '>':
			out = append(out, "&gt;"...)
		case '"':
			out = append(out, "&quot;"...)
		default:
			out = append(out, c)
		}
	}
	return string(out)
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

func PrintTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Repeat("---\t", len(headers)))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}
