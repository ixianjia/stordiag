package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	watchInterval string
	watchExitFail bool
)

type diagSnapshot struct {
	summary string
	layers  map[string]layerSummary
}

type layerSummary struct {
	ok, warn, fail int
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously run diagnostics at intervals",
	Long: `Run diagnostics on a loop, showing compact one-line summaries.
Useful for debugging intermittent issues.

  stordiag watch --interval 30s
  stordiag watch --layers system --interval 5s --exit-on-fail`,
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized")
		}

		interval, err := time.ParseDuration(watchInterval)
		if err != nil {
			return fmt.Errorf("invalid interval %q: %w", watchInterval, err)
		}

		layers := doctorLayers

		fmt.Fprintf(os.Stderr, "=== Watching %s (interval=%s, layers=%s) ===\n\n",
			drv, interval, layers)

		var prev *diagSnapshot
		start := time.Now()
		iteration := 0

		for {
			dr := runDoctor(drv, layers)
			now := time.Now()
			elapsed := now.Sub(start).Round(time.Second)

			curr := &diagSnapshot{
				summary: dr.Summary,
				layers:  make(map[string]layerSummary),
			}
			for _, l := range dr.Layers {
				curr.layers[l.Label] = layerSummary{ok: l.OK, warn: l.WARN, fail: l.FAIL}
			}

			// Build compact output line
			var parts []string
			var changes []string
			for _, l := range dr.Layers {
				part := fmt.Sprintf("%s: OK=%d", l.Label, l.OK)
				if l.WARN > 0 {
					part += fmt.Sprintf(" WARN=%d", l.WARN)
				}
				if l.FAIL > 0 {
					part += fmt.Sprintf(" FAIL=%d", l.FAIL)
				}
				parts = append(parts, part)

				if prev != nil {
					if p, ok := prev.layers[l.Label]; ok {
						if p.ok != l.OK || p.warn != l.WARN || p.fail != l.FAIL {
							changes = append(changes, l.Label)
						}
					}
				}
			}

			marker := ""
			if len(changes) > 0 {
				marker = " ← " + strings.Join(changes, ",")
			}

			line := fmt.Sprintf("[t=%s]  %s  → %s%s",
				elapsed, strings.Join(parts, "  "), curr.summary, marker)

			if dr.Summary == "PASS" {
				fmt.Println(line)
			} else {
				fmt.Fprintln(os.Stderr, line)
			}

			prev = curr
			iteration++

			if watchExitFail && strings.HasPrefix(dr.Summary, "FAIL") {
				return fmt.Errorf("watch: %s", dr.Summary)
			}

			time.Sleep(interval)
		}
	},
}

func init() {
	watchCmd.Flags().StringVar(&watchInterval, "interval", "30s", "Polling interval (e.g. 10s, 1m)")
	watchCmd.Flags().BoolVar(&watchExitFail, "exit-on-fail", false, "Exit immediately on FAIL")
	watchCmd.Flags().StringVar(&doctorLayers, "layers", "all",
		"Layers to probe: all, app, s3, network, system, filesystem")
	rootCmd.AddCommand(watchCmd)
}
