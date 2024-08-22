package upload_utils

import (
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

var (
	defaultPartSize = uint64(1024 * 1024 * 128)
)

type MultipartOpts struct {
	Threads uint
	Size    string
}

func (opt *MultipartOpts) Valid() error {
	if _, err := opt.partSize(); err != nil {
		return errors.Wrap(err, "parse part size")
	}
	return nil
}

func (opt *MultipartOpts) partSize() (uint64, error) {
	if len(opt.Size) == 0 {
		return defaultPartSize, nil
	}
	return humanize.ParseBytes(opt.Size)
}
