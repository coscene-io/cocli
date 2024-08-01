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

package table

import "github.com/samber/lo"

func ColumnDefs2Table[T any](tcdwv []ColumnDefinitionFull[T], data []T, opts *PrintOpts) Table {
	tcdwv = lo.Filter(tcdwv, func(c ColumnDefinitionFull[T], _ int) bool {
		fieldName := c.FieldName
		if c.FieldNameFunc != nil {
			fieldName = c.FieldNameFunc(opts)
		}
		return !lo.Contains(opts.OmitFields, fieldName)
	})
	tcd := lo.Map(tcdwv, func(c ColumnDefinitionFull[T], _ int) ColumnDefinition {
		return c.ToColumnDefinition()
	})

	var rows [][]string
	for _, d := range data {
		var row []string
		for _, c := range tcdwv {
			row = append(row, c.FieldValueFunc(d, opts))
		}
		rows = append(rows, row)
	}

	return Table{
		ColumnDefs: tcd,
		Rows:       rows,
	}
}
