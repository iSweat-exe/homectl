// Package transfer implements chunked file upload/download for the
// daemon. Paths are only validated to be absolute (no sandbox root): the
// Exec RPC already grants full shell access to a paired client, so
// restricting file paths would add no real security, only complexity.
package transfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"homectl/internal/shared/config"
)

// ValidatePath cleans path and rejects anything that is not absolute.
func ValidatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("path must be absolute: %q", path)
	}
	return clean, nil
}

// WriteChunks creates (or truncates) path and writes to it every chunk
// returned by next, until next returns io.EOF, returning the total number
// of bytes written.
func WriteChunks(path string, next func() ([]byte, error)) (uint64, error) {
	clean, err := ValidatePath(path)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		return 0, fmt.Errorf("create parent directories: %w", err)
	}
	f, err := os.OpenFile(clean, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open destination file: %w", err)
	}
	defer f.Close()

	var total uint64
	for {
		data, err := next()
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		n, werr := f.Write(data)
		total += uint64(n)
		if werr != nil {
			return total, fmt.Errorf("write destination file: %w", werr)
		}
	}
}

// ReadChunks reads path in config.TransferChunkSize chunks, invoking send
// for each one in order, until EOF.
func ReadChunks(path string, send func([]byte) error) error {
	clean, err := ValidatePath(path)
	if err != nil {
		return err
	}
	f, err := os.Open(clean)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, config.TransferChunkSize)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if serr := send(buf[:n]); serr != nil {
				return serr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read source file: %w", err)
		}
	}
}
