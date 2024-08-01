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

type ActionRun struct {
	ProjectID string
	ID        string
}

var (
	actionRunNameRe = regroup.MustCompile(`^projects/(?P<project>.*)/actionRuns/(?P<acr>.*)$`)
)

func NewActionRun(acr string) (*ActionRun, error) {
	if match, err := actionRunNameRe.Groups(acr); err != nil {
		return nil, errors.Wrap(err, "parse action run name")
	} else {
		return &ActionRun{ProjectID: match["project"], ID: match["acr"]}, nil
	}
}

func (acr ActionRun) Project() Project {
	return Project{ProjectID: acr.ProjectID}
}

func (acr ActionRun) String() string {
	return fmt.Sprintf("projects/%s/actionRuns/%s", acr.ProjectID, acr.ID)
}
