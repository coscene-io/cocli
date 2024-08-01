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

type Table struct {
	ColumnDefs []ColumnDefinition
	Rows       [][]string
}

type ColumnDefinition struct {
	FieldNameFunc func(*PrintOpts) string
	FieldName     string
	TrimSize      int
}

type ColumnDefinitionFull[T any] struct {
	FieldValueFunc func(T, *PrintOpts) string
	FieldNameFunc  func(*PrintOpts) string
	FieldName      string
	TrimSize       int
}

func (tcd ColumnDefinitionFull[T]) ToColumnDefinition() ColumnDefinition {
	return ColumnDefinition{
		FieldNameFunc: tcd.FieldNameFunc,
		FieldName:     tcd.FieldName,
		TrimSize:      tcd.TrimSize,
	}
}

type PrintOpts struct {
	// Verbose indicates whether to print verbose output.
	Verbose bool

	// OmitFields indicates fields to omit.
	OmitFields []string
}
