package probe

import (
	"context"
	"fmt"
	"time"

	"github.com/chirs/stordiag/internal/driver"
)

type s3ProbeDriver interface {
	driver.Driver
	Bucket() string
	GetBucketVersioning(ctx context.Context) (string, error)
	GetBucketEncryption(ctx context.Context) (string, error)
	ListObjectsCount(ctx context.Context, maxKeys int) (int, time.Duration, error)
}

func ProbeS3(ctx context.Context, drv driver.Driver) *LayerReport {
	r := &LayerReport{Layer: LayerApp}

	s3d, ok := drv.(s3ProbeDriver)
	if !ok {
		r.Add("s3_probes", "N/A", 0, "not an S3 driver", "")
		return r
	}

	// Bucket versioning
	start := time.Now()
	ver, err := s3d.GetBucketVersioning(ctx)
	if err != nil {
		r.Add("s3_versioning", "WARN", time.Since(start),
			fmt.Sprintf("check failed: %v", err), "")
	} else {
		status := "OK"
		if ver == "unversioned" {
			status = "WARN"
		}
		r.Add("s3_versioning", status, time.Since(start),
			ver, "bucket versioning status")
	}

	// Bucket encryption
	start = time.Now()
	enc, err := s3d.GetBucketEncryption(ctx)
	if err != nil {
		r.Add("s3_encryption", "OK", time.Since(start),
			"not configured (no default encryption)", "")
	} else {
		r.Add("s3_encryption", "OK", time.Since(start),
			enc, "default encryption algorithm")
	}

	// Listing performance
	start = time.Now()
	count, listDur, err := s3d.ListObjectsCount(ctx, 100)
	if err != nil {
		r.Add("s3_listing_perf", "WARN", time.Since(start),
			fmt.Sprintf("list failed: %v", err), "")
	} else {
		r.Add("s3_listing_perf", "OK", listDur,
			fmt.Sprintf("%d objects in %s", count, listDur.Round(time.Millisecond)), "")
	}

	return r
}
