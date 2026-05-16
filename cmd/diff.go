package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/report"
)

var diffCmd = &cobra.Command{
	Use:   "diff <baseline.json> <current.json>",
	Short: "Compare two diagnostic reports",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		base, err := loadReport(args[0])
		if err != nil {
			return fmt.Errorf("load baseline: %w", err)
		}
		curr, err := loadReport(args[1])
		if err != nil {
			return fmt.Errorf("load current: %w", err)
		}

		fmt.Printf("=== Diff: %s vs %s ===\n", base.Target, curr.Target)
		fmt.Printf("  Baseline: %s\n", base.Timestamp.Format("15:04:05"))
		fmt.Printf("  Current:  %s\n", curr.Timestamp.Format("15:04:05"))
		fmt.Printf("  Baseline summary: %s\n", base.Summary)
		fmt.Printf("  Current summary:  %s\n\n", curr.Summary)

		allChanged := false
		for _, curLayer := range curr.Layers {
			// find matching baseline layer
			var baseLayer *report.LayerReportItem
			for i := range base.Layers {
				if base.Layers[i].Layer == curLayer.Layer {
					baseLayer = &base.Layers[i]
					break
				}
			}

			label := curLayer.Label
			changed := false

			if baseLayer == nil {
				fmt.Printf("+++ %s (new layer)\n", label)
				for _, p := range curLayer.Probes {
					fmt.Printf("    + %s: %s = %s\n", p.Name, p.Status, p.Value)
				}
				allChanged = true
				continue
			}

			baseProbes := make(map[string]report.LayerProbe)
			for _, p := range baseLayer.Probes {
				baseProbes[p.Name] = p
			}

			var diffs []string
			for _, p := range curLayer.Probes {
				bp, exists := baseProbes[p.Name]
				if !exists {
					diffs = append(diffs, fmt.Sprintf("+%s: %s=%s", p.Name, p.Status, p.Value))
					continue
				}
				if bp.Status != p.Status || bp.Value != p.Value {
					diffs = append(diffs, fmt.Sprintf("~%s: %s→%s (%s→%s)",
						p.Name, bp.Status, p.Status, bp.Value, p.Value))
				}
			}

			if len(diffs) > 0 {
				changed = true
				allChanged = true
			}

			summary := fmt.Sprintf("%s: OK=%d WARN=%d FAIL=%d",
				label, curLayer.OK, curLayer.WARN, curLayer.FAIL)
			if baseLayer.OK != curLayer.OK || baseLayer.WARN != curLayer.WARN || baseLayer.FAIL != curLayer.FAIL {
				summary += fmt.Sprintf(" (was OK=%d WARN=%d FAIL=%d)",
					baseLayer.OK, baseLayer.WARN, baseLayer.FAIL)
			}
			if changed {
				fmt.Printf("~~~ %s\n", summary)
				for _, d := range diffs {
					fmt.Printf("    %s\n", d)
				}
			} else {
				fmt.Printf("    %s (unchanged)\n", summary)
			}
		}

		if !allChanged {
			fmt.Println("\nNo changes detected.")
		}

		return nil
	},
}

func loadReport(path string) (report.DoctorReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return report.DoctorReport{}, err
	}
	var r report.DoctorReport
	if err := json.Unmarshal(data, &r); err != nil {
		return report.DoctorReport{}, err
	}
	return r, nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
