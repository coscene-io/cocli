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

type User struct {
	UserID string
}

var (
	userRe = regroup.MustCompile(`^users\/(?P<user>.*)$`)
)

func NewUser(user string) (*User, error) {
	if match, err := userRe.Groups(user); err != nil {
		return nil, errors.Wrap(err, "parse workflow template name")
	} else {
		return &User{UserID: match["user"]}, nil
	}
}

func (w User) String() string {
	return fmt.Sprintf("users/%s", w.UserID)
}
