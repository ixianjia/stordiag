package checksum

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"

	"github.com/chirs/stordiag/internal/driver"
)

type Kind int

const (
	MD5    Kind = iota
	SHA256
	CRC32C
)

func (k Kind) String() string {
	switch k {
	case MD5:
		return "md5"
	case SHA256:
		return "sha256"
	case CRC32C:
		return "crc32c"
	default:
		return "unknown"
	}
}

type Result struct {
	Kind   Kind
	Digest string
	Size   int64
}

func Compute(r io.Reader, kind Kind) (Result, error) {
	var h hash.Hash
	switch kind {
	case MD5:
		h = md5.New()
	case SHA256:
		h = sha256.New()
	default:
		return Result{}, fmt.Errorf("unsupported checksum kind: %v", kind)
	}
	n, err := io.Copy(h, r)
	if err != nil {
		return Result{}, fmt.Errorf("checksum: %w", err)
	}
	return Result{
		Kind:   kind,
		Digest: hex.EncodeToString(h.Sum(nil)),
		Size:   n,
	}, nil
}

type Verifier struct {
	Driver driver.Driver
}

func (v *Verifier) Verify(ctx context.Context, path string, kind Kind) (Result, Result, bool, error) {
	// read existing data
	var buf bytes.Buffer
	if err := v.Driver.Read(ctx, path, &buf); err != nil {
		return Result{}, Result{}, false, fmt.Errorf("read: %w", err)
	}

	original := buf.Bytes()
	remote, err := Compute(bytes.NewReader(original), kind)
	if err != nil {
		return Result{}, Result{}, false, err
	}

	// re-read to verify
	var buf2 bytes.Buffer
	if err := v.Driver.Read(ctx, path, &buf2); err != nil {
		return Result{}, Result{}, false, fmt.Errorf("re-read: %w", err)
	}

	local, err := Compute(bytes.NewReader(buf2.Bytes()), kind)
	if err != nil {
		return Result{}, Result{}, false, err
	}

	ok := bytes.Equal(original, buf2.Bytes())
	return remote, local, ok, nil
}

func (v *Verifier) VerifyEndToEnd(ctx context.Context, path string, kind Kind, data []byte) (bool, error) {
	// write known data, read back, compare checksums
	if err := v.Driver.Write(ctx, path, bytes.NewReader(data), int64(len(data))); err != nil {
		return false, fmt.Errorf("write: %w", err)
	}

	var buf bytes.Buffer
	if err := v.Driver.Read(ctx, path, &buf); err != nil {
		return false, fmt.Errorf("read: %w", err)
	}

	readback, err := Compute(bytes.NewReader(buf.Bytes()), kind)
	if err != nil {
		return false, err
	}

	expected, err := Compute(bytes.NewReader(data), kind)
	if err != nil {
		return false, err
	}

	return readback.Digest == expected.Digest, nil
}
