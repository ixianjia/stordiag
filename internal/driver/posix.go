package driver

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type POSIXDriver struct {
	root string
}

func NewPOSIXDriver(root string) (*POSIXDriver, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	return &POSIXDriver{root: abs}, nil
}

func (d *POSIXDriver) Type() string { return "posix" }

func (d *POSIXDriver) String() string { return d.root }

func (d *POSIXDriver) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(d.root, path)
}

func (d *POSIXDriver) Ping(ctx context.Context) HealthResult {
	start := time.Now()
	_, err := os.Stat(d.root)
	latency := time.Since(start)
	if err != nil {
		return HealthResult{Reachable: false, Latency: latency, Error: err.Error()}
	}
	return HealthResult{Reachable: true, Latency: latency}
}

func (d *POSIXDriver) Stat(ctx context.Context, path string) (Stat, error) {
	fi, err := os.Stat(d.resolve(path))
	if err != nil {
		return Stat{}, fmt.Errorf("stat: %w", err)
	}
	mode := "file"
	if fi.IsDir() {
		mode = "dir"
	}
	return Stat{
		Name:  path,
		Size:  fi.Size(),
		IsDir: fi.IsDir(),
		Mode:  mode,
		Mtime: fi.ModTime(),
	}, nil
}

func (d *POSIXDriver) List(ctx context.Context, prefix string) ([]Stat, error) {
	entries, err := os.ReadDir(d.resolve(prefix))
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	var out []Stat
	for _, e := range entries {
		fi, _ := e.Info()
		mtime := time.Time{}
		mode := "file"
		if fi != nil {
			mtime = fi.ModTime()
		}
		if e.IsDir() {
			mode = "dir"
		}
		var size int64
		if fi != nil {
			size = fi.Size()
		}
		out = append(out, Stat{
			Name:  filepath.Join(prefix, e.Name()),
			Size:  size,
			IsDir: e.IsDir(),
			Mode:  mode,
			Mtime: mtime,
		})
	}
	return out, nil
}

func (d *POSIXDriver) Read(ctx context.Context, path string, w io.Writer) error {
	f, err := os.Open(d.resolve(path))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func (d *POSIXDriver) Delete(ctx context.Context, path string) error {
	if err := os.Remove(d.resolve(path)); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}

func (d *POSIXDriver) Write(ctx context.Context, path string, r io.Reader, size int64) error {
	full := d.resolve(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(full)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
