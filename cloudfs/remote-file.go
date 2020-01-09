package cloudfs

import (
	"context"

	"gocloud.dev/blob"
)

type remoteFile struct {
	path   string
	ctx    context.Context
	bucket *blob.Bucket
}

type remoteFileWriter struct {
	writer *blob.Writer
}

func (f *remoteFile) ReadAt(p []byte, off int64) (int, error) {
	r, err := f.bucket.NewRangeReader(f.ctx, f.path, off, int64(len(p)), nil)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return r.Read(p)
}

func (w *remoteFileWriter) WriteAt(p []byte, off int64) (int, error) {
	n, err := w.writer.Write(p)
	return n, err
}

func (w *remoteFileWriter) Close() error {
	return w.writer.Close()
}

func (f *remoteFile) Attributes() (*blob.Attributes, error) {
	return f.bucket.Attributes(f.ctx, f.path)
}
