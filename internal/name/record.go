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

type Record struct {
	ProjectID string
	RecordID  string
}

var (
	recordRe = regroup.MustCompile(`^projects/(?P<project>.*)/records/(?P<record>.*)$`)
)

func NewRecord(record string) (*Record, error) {
	if match, err := recordRe.Groups(record); err != nil {
		return nil, errors.Wrap(err, "parse record name")
	} else {
		return &Record{ProjectID: match["project"], RecordID: match["record"]}, nil
	}
}

func (r Record) Project() *Project {
	return &Project{ProjectID: r.ProjectID}
}

func (r Record) String() string {
	return fmt.Sprintf("projects/%s/records/%s", r.ProjectID, r.RecordID)
}
