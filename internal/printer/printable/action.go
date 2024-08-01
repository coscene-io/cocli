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
	"time"

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/internal/printer/table"
	"google.golang.org/protobuf/proto"
)

const (
	actionIdTrimSize       = 36
	actionCategoryTrimSize = 7
	actionTitleTrimSize    = 30
	actionAuthorTrimSize   = 20
	actionTimeTrimSize     = len(time.RFC3339)
)

type Action struct {
	Delegate []*openv1alpha1resource.Action
}

func NewAction(actions []*openv1alpha1resource.Action) *Action {
	return &Action{
		Delegate: actions,
	}
}

func (p *Action) ToProtoMessage() proto.Message {
	return &openv1alpha1service.ListActionsResponse{
		Actions:   p.Delegate,
		TotalSize: int64(len(p.Delegate)),
	}
}

func (p *Action) ToTable(opts *table.PrintOpts) table.Table {
	fullColumnDefs := []table.ColumnDefinitionFull[*openv1alpha1resource.Action]{
		{
			FieldNameFunc: func(opts *table.PrintOpts) string {
				if opts.Verbose {
					return "RESOURCE NAME"
				}
				return "ID"
			},
			FieldValueFunc: func(a *openv1alpha1resource.Action, opts *table.PrintOpts) string {
				if opts.Verbose {
					return a.Name
				}
				actionName, _ := name.NewAction(a.Name)
				return actionName.ID
			},
			TrimSize: actionIdTrimSize,
		},
		{
			FieldName: "CATEGORY",
			FieldValueFunc: func(a *openv1alpha1resource.Action, opts *table.PrintOpts) string {
				actionName, _ := name.NewAction(a.Name)
				if actionName.IsWftmpl() {
					return "system"
				}
				return "custom"
			},
			TrimSize: actionCategoryTrimSize,
		},
		{
			FieldName: "TITLE",
			FieldValueFunc: func(a *openv1alpha1resource.Action, opts *table.PrintOpts) string {
				return a.Spec.Name
			},
			TrimSize: actionTitleTrimSize,
		},
		{
			FieldName: "AUTHOR",
			FieldValueFunc: func(a *openv1alpha1resource.Action, opts *table.PrintOpts) string {
				return a.Author
			},
			TrimSize: actionAuthorTrimSize,
		},
		{
			FieldName: "UPDATE TIME",
			FieldValueFunc: func(a *openv1alpha1resource.Action, opts *table.PrintOpts) string {
				return a.UpdateTime.AsTime().In(time.Local).Format(time.RFC3339)
			},
			TrimSize: actionTimeTrimSize,
		},
	}

	return table.ColumnDefs2Table(fullColumnDefs, p.Delegate, opts)
}
