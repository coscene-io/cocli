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

package name

import (
	"fmt"

	"github.com/oriser/regroup"
	"github.com/pkg/errors"
)

type File struct {
	ProjectID string
	RecordID  string
	Filename  string
}

var (
	fileRe = regroup.MustCompile(`^projects/(?P<project>.*)/records/(?P<record>.*)/files/(?P<file>.*)$`)
)

func NewFile(file string) (*File, error) {
	if match, err := fileRe.Groups(file); err != nil {
		return nil, errors.Wrap(err, "parse file name")
	} else {
		return &File{ProjectID: match["project"], RecordID: match["record"], Filename: match["file"]}, nil
	}
}

func (f File) Project() Project {
	return Project{ProjectID: f.ProjectID}
}

func (f File) String() string {
	return fmt.Sprintf("projects/%s/records/%s/files/%s", f.ProjectID, f.RecordID, f.Filename)
}
