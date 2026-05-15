package driver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Driver struct {
	client *minio.Client
	bucket string
	ep     string
}

func NewS3Driver(endpoint, accessKey, secretKey, bucket string, secure bool) (*S3Driver, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}
	return &S3Driver{client: client, bucket: bucket, ep: endpoint}, nil
}

func (d *S3Driver) Type() string { return "s3" }

func (d *S3Driver) String() string {
	return fmt.Sprintf("s3://%s@%s", d.bucket, d.ep)
}

func (d *S3Driver) Ping(ctx context.Context) HealthResult {
	start := time.Now()
	_, err := d.client.ListBuckets(ctx)
	latency := time.Since(start)
	if err != nil {
		return HealthResult{Reachable: false, Latency: latency, Error: err.Error()}
	}
	return HealthResult{Reachable: true, Latency: latency}
}

func (d *S3Driver) Stat(ctx context.Context, path string) (Stat, error) {
	obj, err := d.client.StatObject(ctx, d.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		return Stat{}, fmt.Errorf("stat object: %w", err)
	}
	return Stat{
		Name:  obj.Key,
		Size:  obj.Size,
		ETag:  obj.ETag,
		Mtime: obj.LastModified,
	}, nil
}

func (d *S3Driver) List(ctx context.Context, prefix string) ([]Stat, error) {
	var out []Stat
	for obj := range d.client.ListObjects(ctx, d.bucket, minio.ListObjectsOptions{Prefix: prefix}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		out = append(out, Stat{
			Name:  obj.Key,
			Size:  obj.Size,
			ETag:  obj.ETag,
			Mtime: obj.LastModified,
		})
	}
	return out, nil
}

func (d *S3Driver) Read(ctx context.Context, path string, w io.Writer) error {
	obj, err := d.client.GetObject(ctx, d.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer obj.Close()
	_, err = io.Copy(w, obj)
	return err
}

func (d *S3Driver) Write(ctx context.Context, path string, r io.Reader, size int64) error {
	_, err := d.client.PutObject(ctx, d.bucket, path, r, size,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func (d *S3Driver) Bucket() string { return d.bucket }

func (d *S3Driver) Endpoint() string { return d.ep }

func (d *S3Driver) parseS3URL(raw string) (bucket, prefix string) {
	raw = strings.TrimPrefix(raw, "s3://")
	parts := strings.SplitN(raw, "/", 2)
	bucket = parts[0]
	if len(parts) > 1 {
		prefix = parts[1]
	}
	// write empty content to create bucket placeholder for stat
	if bucket != "" && d.bucket == "" {
		d.bucket = bucket
	}
	return
}

func (d *S3Driver) ensureBucket(ctx context.Context) error {
	exists, err := d.client.BucketExists(ctx, d.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return d.client.MakeBucket(ctx, d.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

func (d *S3Driver) benchOnce(ctx context.Context, path string, buf []byte) (time.Duration, error) {
	start := time.Now()
	err := d.Write(ctx, path, bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return 0, err
	}
	return time.Since(start), nil
}
