package cloudfs

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/eikenb/pipeat"
	"gocloud.dev/blob"
)

type remoteFile struct {
	path   string
	ctx    context.Context
	bucket *blob.Bucket
}

type remoteFileWriter struct {
	readerAt *pipeat.PipeReaderAt
	writerAt *pipeat.PipeWriterAt
	writer   *blob.Writer
}

func newRemoteFileWriter(ctx context.Context, b *blob.Bucket, key string) (*remoteFileWriter, error) {
	readerAt, writerAt, err := pipeat.Pipe()
	if err != nil {
		return nil, err
	}

	writer, err := b.NewWriter(ctx, key, nil)
	if err != nil {
		return nil, err
	}

	go func() {
		defer readerAt.Close()
		for {
			p := make([]byte, defaultChunkSize)
			bytesRead, err := readerAt.Read(p)
			if err == nil || err == io.EOF {
				_, writeErr := writer.Write(p[:bytesRead])
				if writeErr != nil {
					break
				}
			}

			if err != nil && err != io.EOF {
				break
			}

			if err == io.EOF {
				break
			}
		}
	}()

	return &remoteFileWriter{
		writer:   writer,
		readerAt: readerAt,
		writerAt: writerAt,
	}, nil
}

func (f *remoteFile) ReadAt(p []byte, off int64) (int, error) {
	fmt.Printf("Read At off: %v, len: %v\n", off, len(p))
	r, err := f.bucket.NewRangeReader(f.ctx, f.path, off, int64(len(p)), nil)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return r.Read(p)
}

var defaultChunkSize = 32768

func (w *remoteFileWriter) WriteAt(p []byte, off int64) (int, error) {
	fmt.Printf("Write At off: %v, len: %v\n", off, len(p))
	i, err := w.writerAt.WriteAt(p, off)
	if err != nil {
		return i, err
	}
	return i, nil
}

func (w *remoteFileWriter) Close() error {
	writerAtErr := w.writerAt.Close()
	w.writerAt.WaitForReader()
	writerErr := w.writer.Close()
	if writerAtErr != nil || writerErr != nil {
		return errors.New("Failed to upload file")
	}
	return nil
}

func (f *remoteFile) Attributes() (*blob.Attributes, error) {
	return f.bucket.Attributes(f.ctx, f.path)
}
