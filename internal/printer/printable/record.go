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
	"strconv"
	"strings"
	"time"

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/internal/printer/table"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
)

const (
	recordIdTrimSize      = 36
	recordArchiveTrimSize = 8
	recordTitleTrimSize   = 40
	recordLabelsTrimSize  = 25
	recordTimeTrimSize    = len(time.RFC3339)
)

type Record struct {
	Delegate []*openv1alpha1resource.Record
}

func NewRecord(records []*openv1alpha1resource.Record) *Record {
	return &Record{
		Delegate: records,
	}
}

func (p *Record) ToProtoMessage() proto.Message {
	return &openv1alpha1service.ListRecordsResponse{
		Records:   p.Delegate,
		TotalSize: int64(len(p.Delegate)),
	}
}

func (p *Record) ToTable(opts *table.PrintOpts) table.Table {
	fullColumnDefs := []table.ColumnDefinitionFull[*openv1alpha1resource.Record]{
		{
			FieldNameFunc: func(opts *table.PrintOpts) string {
				if opts.Verbose {
					return "RESOURCE NAME"
				}
				return "ID"
			},
			FieldValueFunc: func(r *openv1alpha1resource.Record, opts *table.PrintOpts) string {
				if opts.Verbose {
					return r.Name
				}
				recordName, _ := name.NewRecord(r.Name)
				return recordName.RecordID
			},
			TrimSize: recordIdTrimSize,
		},
		{
			FieldName: "ARCHIVED",
			FieldValueFunc: func(r *openv1alpha1resource.Record, opts *table.PrintOpts) string {
				return strconv.FormatBool(r.IsArchived)
			},
			TrimSize: recordArchiveTrimSize,
		},
		{
			FieldName: "TITLE",
			FieldValueFunc: func(r *openv1alpha1resource.Record, opts *table.PrintOpts) string {
				return r.Title
			},
			TrimSize: recordTitleTrimSize,
		},
		{
			FieldName: "LABELS",
			FieldValueFunc: func(r *openv1alpha1resource.Record, opts *table.PrintOpts) string {
				labels := lo.Map(r.Labels, func(l *openv1alpha1resource.Label, _ int) string {
					return l.DisplayName
				})
				return strings.Join(labels, ", ")
			},
			TrimSize: recordLabelsTrimSize,
		},
		{
			FieldName: "CREATE TIME",
			FieldValueFunc: func(r *openv1alpha1resource.Record, opts *table.PrintOpts) string {
				return r.CreateTime.AsTime().In(time.Local).Format(time.RFC3339)
			},
			TrimSize: recordTimeTrimSize,
		},
	}

	return table.ColumnDefs2Table(fullColumnDefs, p.Delegate, opts)
}
