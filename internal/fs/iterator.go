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
	"os"
	"path/filepath"
	"strings"

	"github.com/coscene-io/cocli/internal/constants"
	log "github.com/sirupsen/logrus"
)

// GenerateFiles generates a channel of file paths in the given directory.
// It will walk through the directory and return the absolute path of each file.
// Note that if root is a file, it will return the file itself.
//
// If isRecursive is true, it will walk through all subdirectories.
// Otherwise, it will only walk through the top level directory.
//
// If includeHidden is true, it will include hidden files (files starting with a dot).
// Otherwise, it will skip hidden files.
func GenerateFiles(root string, isRecursive, includeHidden bool) <-chan string {
	c := make(chan string)

	go func() {
		defer close(c)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip the .[constants.CLIName] directory
			if d.IsDir() && d.Name() == "."+constants.CLIName {
				return filepath.SkipDir
			}

			// Skip hidden files if not includeHidden
			if !includeHidden && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				} else {
					return nil
				}
			}

			// skip directories if not recursive
			if d.IsDir() && !isRecursive && path != root {
				return filepath.SkipDir
			}

			if !d.IsDir() {
				c <- path
			}

			return nil
		})
		if err != nil {
			log.Errorf("unable to walk through directory: %v", err)
			return
		}
	}()

	return c
}
