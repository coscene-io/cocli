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

package printer

import (
	"io"

	"github.com/coscene-io/cocli/internal/printer/printable"
	"github.com/coscene-io/cocli/internal/printer/table"
)

type Interface interface {
	// PrintObj prints the object to the writer.
	PrintObj(obj printable.Interface, w io.Writer) error
}

type Options struct {
	TableOpts *table.PrintOpts
}

func Printer(format string, opts *Options) Interface {
	switch format {
	case "json":
		return &JSONPrinter{}
	default:
		return &TablePrinter{Opts: opts.TableOpts}
	}
}
