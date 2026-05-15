package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/report"
)

var healthCmd = &cobra.Command{
	Use:   "health [target]",
	Short: "Run health check against storage backend",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized; use --endpoint or STORDIAG_ENDPOINT")
		}

		ctx, cancel := ctx()
		defer cancel()

		result := drv.Ping(ctx)
		hr := report.HealthReport{
			Target:    drv.String(),
			Type:      drv.Type(),
			Reachable: result.Reachable,
			Latency:   result.Latency,
			Error:     result.Error,
			Timestamp: time.Now(),
		}

		if result.Reachable {
			// additional checks
			if st, err := drv.Stat(ctx, ""); err == nil {
				hr.Checks = append(hr.Checks, report.CheckItem{
					Name:   "root_stat",
					Status: "OK",
					Value:  report.FormatBytes(st.Size),
				})
			} else {
				hr.Checks = append(hr.Checks, report.CheckItem{
					Name:   "root_stat",
					Status: "OK",
					Value:  "accessible",
				})
			}

			entries, err := drv.List(ctx, "")
			if err == nil {
				hr.Checks = append(hr.Checks, report.CheckItem{
					Name:   "list_objects",
					Status: "OK",
					Value:  fmt.Sprintf("%d objects", len(entries)),
				})
			} else {
				hr.Checks = append(hr.Checks, report.CheckItem{
					Name:   "list_objects",
					Status: "WARN",
					Value:  err.Error(),
				})
			}
		}

		f := report.FormatText
		if jsonOut {
			f = report.FormatJSON
		}
		report.PrintHealth(os.Stdout, hr, f)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
