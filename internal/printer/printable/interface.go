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

package printable

import (
	"github.com/coscene-io/cocli/internal/printer/table"
	"google.golang.org/protobuf/proto"
)

type jsonPrintable interface {
	ToProtoMessage() proto.Message
}

type tablePrintable interface {
	ToTable(*table.PrintOpts) table.Table
}

type Interface interface {
	jsonPrintable
	tablePrintable
}
