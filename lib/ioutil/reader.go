package ioutil

import (
	"context"
	"io"
)

type limitReader struct {
	cancel context.CancelFunc
	reader io.Reader
	i      int64
	limit  int64
}

type LimitReader interface {
	io.Reader
	HasExceededLimit() bool
}

func NewLimitReader(cancel context.CancelFunc, reader io.Reader, limit int64) LimitReader {
	return &limitReader{cancel: cancel, reader: reader, limit: limit, i: 0}
}

func (r *limitReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.i += int64(n)
	if r.HasExceededLimit() {
		r.cancel()
	}
	return n, err
}

func (r *limitReader) HasExceededLimit() bool {
	return r.i > r.limit
}
