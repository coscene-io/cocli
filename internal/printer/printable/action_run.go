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
	actionRunIdTrimSize          = 36
	actionRunStateTrimSize       = 10
	actionRunActionTitleTrimSize = 30
	actionRunTimeTrimSize        = len(time.RFC3339)
	actionRunCreatorTrimSize     = 20
)

type ActionRun struct {
	Delegate []*openv1alpha1resource.ActionRun
}

func NewActionRun(actionRuns []*openv1alpha1resource.ActionRun) *ActionRun {
	return &ActionRun{
		Delegate: actionRuns,
	}
}

func (p *ActionRun) ToProtoMessage() proto.Message {
	return &openv1alpha1service.ListActionRunsResponse{
		ActionRuns: p.Delegate,
		TotalSize:  int64(len(p.Delegate)),
	}
}

func (p *ActionRun) ToTable(opts *table.PrintOpts) table.Table {
	fullColumnDefs := []table.ColumnDefinitionFull[*openv1alpha1resource.ActionRun]{
		{
			FieldNameFunc: func(opts *table.PrintOpts) string {
				if opts.Verbose {
					return "RESOURCE NAME"
				}
				return "ID"
			},
			FieldValueFunc: func(a *openv1alpha1resource.ActionRun, opts *table.PrintOpts) string {
				if opts.Verbose {
					return a.Name
				}
				actionRunName, _ := name.NewActionRun(a.Name)
				return actionRunName.ID
			},
			TrimSize: actionRunIdTrimSize,
		},
		{
			FieldName: "STATE",
			FieldValueFunc: func(a *openv1alpha1resource.ActionRun, opts *table.PrintOpts) string {
				return a.State.String()
			},
			TrimSize: actionRunStateTrimSize,
		},
		{
			FieldName: "ACTION TITLE",
			FieldValueFunc: func(a *openv1alpha1resource.ActionRun, opts *table.PrintOpts) string {
				return a.Action.Spec.Name
			},
			TrimSize: actionRunActionTitleTrimSize,
		},
		{
			FieldName: "CREATE TIME",
			FieldValueFunc: func(a *openv1alpha1resource.ActionRun, opts *table.PrintOpts) string {
				return a.CreateTime.AsTime().In(time.Local).Format(time.RFC3339)
			},
			TrimSize: actionRunTimeTrimSize,
		},
		{
			FieldName: "CREATOR",
			FieldValueFunc: func(a *openv1alpha1resource.ActionRun, opts *table.PrintOpts) string {
				switch t := a.Creator.(type) {
				case *openv1alpha1resource.ActionRun_User:
					return t.User
				case *openv1alpha1resource.ActionRun_Trigger:
					return "trigger: " + t.Trigger.Spec.Name
				}
				return ""
			},
			TrimSize: actionRunCreatorTrimSize,
		},
	}

	return table.ColumnDefs2Table(fullColumnDefs, p.Delegate, opts)
}
