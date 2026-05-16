package driver

import (
	"context"
	"fmt"
	"io"
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

func (d *S3Driver) Delete(ctx context.Context, path string) error {
	return d.client.RemoveObject(ctx, d.bucket, path, minio.RemoveObjectOptions{})
}

func (d *S3Driver) Write(ctx context.Context, path string, r io.Reader, size int64) error {
	_, err := d.client.PutObject(ctx, d.bucket, path, r, size,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func (d *S3Driver) Bucket() string { return d.bucket }

func (d *S3Driver) Endpoint() string { return d.ep }

func (d *S3Driver) EnsureBucket(ctx context.Context) error {
	exists, err := d.client.BucketExists(ctx, d.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return d.client.MakeBucket(ctx, d.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

func (d *S3Driver) GetBucketVersioning(ctx context.Context) (string, error) {
	v, err := d.client.GetBucketVersioning(ctx, d.bucket)
	if err != nil {
		return "", err
	}
	if v.Status == "" {
		return "unversioned", nil
	}
	return v.Status, nil
}

func (d *S3Driver) GetBucketEncryption(ctx context.Context) (string, error) {
	cfg, err := d.client.GetBucketEncryption(ctx, d.bucket)
	if err != nil {
		return "", err
	}
	if cfg == nil {
		return "none", nil
	}
	if len(cfg.Rules) == 0 {
		return "none", nil
	}
	return cfg.Rules[0].Apply.SSEAlgorithm, nil
}

func (d *S3Driver) ListObjectsCount(ctx context.Context, maxKeys int) (int, time.Duration, error) {
	start := time.Now()
	opts := minio.ListObjectsOptions{
		MaxKeys: maxKeys,
	}
	var count int
	for obj := range d.client.ListObjects(ctx, d.bucket, opts) {
		if obj.Err != nil {
			return count, time.Since(start), obj.Err
		}
		count++
	}
	return count, time.Since(start), nil
}


