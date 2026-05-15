package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/probe"
	"github.com/chirs/stordiag/internal/report"
)

var layersCmd = &cobra.Command{
	Use:   "layers [app|network|system]",
	Short: "Run diagnostics for a specific layer",
	Long: `Run diagnostics for a single layer:
  app     — health, bench, data integrity
  network — DNS, TCP connect, TLS breakdown
  system  — disk IO, memory, CPU iowait, pressure stall`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized")
		}

		ctx, cancel := ctx()
		defer cancel()

		var lr *probe.LayerReport
		switch args[0] {
		case "app":
			lr = runAppLayer(ctx, drv)
		case "network":
			lr = probe.ProbeNetwork(ctx, drv)
		case "system":
			lr = probe.ProbeSystem(ctx, drv)
		default:
			return fmt.Errorf("unknown layer: %s (use app, network, or system)", args[0])
		}

		item := report.LayerReportItem{
			Layer:  fmt.Sprintf("L%d:%s", lr.Layer, args[0]),
			Label:  args[0],
			Probes: make([]report.LayerProbe, len(lr.Results)),
		}
		item.OK, item.WARN, item.FAIL = lr.Summary()
		for i, p := range lr.Results {
			item.Probes[i] = report.LayerProbe{
				Name: p.Name, Status: p.Status,
				Latency: durationStr(p.Latency),
				Value:   p.Value, Detail: p.Detail,
			}
		}

		f := report.FormatText
		if jsonOut {
			f = report.FormatJSON
		}
		report.PrintLayerOutput(os.Stdout, item, f)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(layersCmd)
}
