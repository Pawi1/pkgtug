package compress

import (
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Algo identifies a compression algorithm.
type Algo string

const (
	None Algo = ""
	Zstd Algo = "zstd"
	XZ   Algo = "xz"
)

// NewWriter wraps w with a compressor for the given algo.
// The caller must call Close() on the returned WriteCloser.
func NewWriter(w io.Writer, algo Algo) (io.WriteCloser, error) {
	switch algo {
	case None:
		return nopCloser{w}, nil
	case Zstd:
		enc, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
		if err != nil {
			return nil, err
		}
		return enc, nil
	case XZ:
		xw, err := xz.NewWriter(w)
		if err != nil {
			return nil, err
		}
		return xw, nil
	default:
		return nil, fmt.Errorf("unknown compression algorithm: %q", algo)
	}
}

// NewReader wraps r with a decompressor for the given algo.
func NewReader(r io.Reader, algo Algo) (io.ReadCloser, error) {
	switch algo {
	case None:
		return io.NopCloser(r), nil
	case Zstd:
		dec, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return dec.IOReadCloser(), nil
	case XZ:
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(xr), nil
	default:
		return nil, fmt.Errorf("unknown compression algorithm: %q", algo)
	}
}

// Extension returns the file extension for the algorithm.
func Extension(algo Algo) string {
	switch algo {
	case Zstd:
		return ".zst"
	case XZ:
		return ".xz"
	default:
		return ""
	}
}

// Parse normalises user-provided algorithm name.
func Parse(s string) (Algo, error) {
	switch s {
	case "", "none":
		return None, nil
	case "zstd":
		return Zstd, nil
	case "xz":
		return XZ, nil
	default:
		return None, fmt.Errorf("unsupported compression %q: use zstd or xz", s)
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
