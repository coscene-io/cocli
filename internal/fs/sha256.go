// Copyright 2024 coScene
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fs

import (
	"fmt"
	"io"
	"os"

	"github.com/minio/sha256-simd"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// CalSha256AndSize calculates the sha256 hash and size of the file at the given path.
func CalSha256AndSize(absPath string) (string, int64, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return "", 0, errors.Wrapf(err, "open file")
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Error(err)
		}
	}(f)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", 0, errors.Wrapf(err, "read file")
	}

	fileInfo, err := f.Stat()
	if err != nil {
		return "", 0, errors.Wrapf(err, "stat file")
	}

	return fmt.Sprintf("%x", h.Sum(nil)), fileInfo.Size(), nil
}
