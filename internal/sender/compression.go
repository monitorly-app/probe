package sender

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// compressWriter is an interface that wraps io.WriteCloser and adds a Bytes method
type compressWriter interface {
	io.WriteCloser
	Bytes() []byte
}

// gzipWriter implements the compressWriter interface
type gzipWriter struct {
	buf io.Writer
	gw  *gzip.Writer
}

// Write implements io.Writer
func (w *gzipWriter) Write(p []byte) (n int, err error) {
	return w.gw.Write(p)
}

// Close implements io.Closer
func (w *gzipWriter) Close() error {
	return w.gw.Close()
}

// Bytes returns the compressed data if the underlying writer is a *bytes.Buffer
func (w *gzipWriter) Bytes() []byte {
	if buf, ok := w.buf.(*bytes.Buffer); ok {
		return buf.Bytes()
	}
	return nil
}

// writerFactory is a function type that creates a new compressWriter
type writerFactory func(io.Writer) compressWriter

// defaultGzipWriterFactory creates a new gzipWriter with default compression
func defaultGzipWriterFactory(w io.Writer) compressWriter {
	return &gzipWriter{
		buf: w,
		gw:  gzip.NewWriter(w),
	}
}

// compressData compresses data using gzip compression
func compressData(data []byte) ([]byte, error) {
	return compressDataWithFactory(data, defaultGzipWriterFactory)
}

// compressDataWithFactory compresses data using the provided writer factory
func compressDataWithFactory(data []byte, factory writerFactory) ([]byte, error) {
	var buf bytes.Buffer
	writer := factory(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// compressDataWithWriter compresses data using the provided writer
func compressDataWithWriter(data []byte, writer compressWriter) ([]byte, error) {
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	result := writer.Bytes()
	if result == nil {
		// If writer.Bytes() returns nil, return an empty byte slice
		return []byte{}, nil
	}

	return result, nil
}
