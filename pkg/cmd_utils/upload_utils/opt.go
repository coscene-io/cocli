package upload_utils

import (
	"os"
	"path/filepath"

	"github.com/coscene-io/cocli/api"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

type ApiOpts struct {
	api.SecurityTokenInterface
	api.FileInterface
}

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

type FileOpts struct {
	Path          string
	relDir        string
	Recursive     bool
	IncludeHidden bool
}

func (opt *FileOpts) Valid() error {
	if opt.Path == "" {
		return errors.New("file path not empty")
	}

	opt.relDir = opt.Path
	fileInfo, err := os.Stat(opt.Path)
	if err != nil {
		return errors.Wrap(err, "invalid file path")
	}
	if !fileInfo.IsDir() {
		opt.relDir = filepath.Dir(opt.Path)
	}
	return nil
}
