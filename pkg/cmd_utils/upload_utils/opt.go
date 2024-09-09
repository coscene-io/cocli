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

type UploadManagerOpts struct {
	Threads        int
	PartSize       string
	partSizeUint64 uint64
}

func (opt *UploadManagerOpts) Valid() error {
	if sizeUint64, err := opt.partSize(); err != nil {
		return errors.Wrap(err, "parse part size")
	} else {
		opt.partSizeUint64 = sizeUint64
		return nil
	}
}

func (opt *UploadManagerOpts) partSize() (uint64, error) {
	if len(opt.PartSize) == 0 {
		return defaultPartSize, nil
	}
	return humanize.ParseBytes(opt.PartSize)
}

type FileOpts struct {
	Path          string
	relDir        string
	Recursive     bool
	IncludeHidden bool

	// Additional mapping from file path to oss path
	AdditionalUploads map[string]string
}

func (opt *FileOpts) Valid() error {
	if opt.Path == "" && len(opt.AdditionalUploads) == 0 {
		return errors.New("file path empty")
	}

	if opt.Path == "" {
		return nil
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
