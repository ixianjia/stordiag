package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/chirs/stordiag/internal/checksum"
	"github.com/chirs/stordiag/internal/report"
)

var verifyAlgo string

var verifyCmd = &cobra.Command{
	Use:   "verify <path>",
	Short: "Verify data integrity via end-to-end checksum",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		drv := globalDriver
		if drv == nil {
			return fmt.Errorf("no driver initialized")
		}

		path := args[0]

		var kind checksum.Kind
		switch verifyAlgo {
		case "md5":
			kind = checksum.MD5
		case "sha256":
			kind = checksum.SHA256
		default:
			return fmt.Errorf("unsupported algorithm: %s (use md5 or sha256)", verifyAlgo)
		}

		ctx, cancel := ctx()
		defer cancel()

		// Generate random test data
		data := make([]byte, 1<<20) // 1 MiB
		if _, err := rand.Read(data); err != nil {
			return fmt.Errorf("generate data: %w", err)
		}

		v := &checksum.Verifier{Driver: drv}
		ok, err := v.VerifyEndToEnd(ctx, path, kind, data)
		if err != nil {
			return fmt.Errorf("verify: %w", err)
		}

		// read back and compute for reporting
		remote, local, match, err := v.Verify(ctx, path, kind)
		if err != nil {
			return fmt.Errorf("re-verify: %w", err)
		}

		vr := report.VerifyReport{
			Target:     drv.String(),
			Type:       drv.Type(),
			Path:       path,
			Algo:       kind.String(),
			RemoteHash: remote.Digest,
			LocalHash:  local.Digest,
			Match:      ok && match,
			Timestamp:  time.Now(),
		}

		f := report.FormatText
		if jsonOut {
			f = report.FormatJSON
		}
		report.PrintVerify(os.Stdout, vr, f)
		return nil
	},
}

func init() {
	verifyCmd.Flags().StringVar(&verifyAlgo, "algo", "sha256", "Checksum algorithm (md5|sha256)")
	rootCmd.AddCommand(verifyCmd)
}
