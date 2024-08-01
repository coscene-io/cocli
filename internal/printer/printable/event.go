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
	"github.com/coscene-io/cocli/internal/printer/table"
	"google.golang.org/protobuf/proto"
)

const (
	eventNameTrimSize = 20
	eventTimeTrimSize = len(time.RFC3339)
)

type Event struct {
	Delegate []*openv1alpha1resource.Event
}

func NewEvent(events []*openv1alpha1resource.Event) *Event {
	return &Event{
		Delegate: events,
	}
}

func (p *Event) ToProtoMessage() proto.Message {
	return &openv1alpha1service.ListRecordEventsResponse{
		Events:    p.Delegate,
		TotalSize: int64(len(p.Delegate)),
	}
}

func (p *Event) ToTable(opts *table.PrintOpts) table.Table {
	fullColumnDefs := []table.ColumnDefinitionFull[*openv1alpha1resource.Event]{
		{
			FieldName: "NAME",
			FieldValueFunc: func(e *openv1alpha1resource.Event, opts *table.PrintOpts) string {
				return e.DisplayName
			},
			TrimSize: eventNameTrimSize,
		},
		{
			FieldName: "TRIGGER TIME",
			FieldValueFunc: func(e *openv1alpha1resource.Event, opts *table.PrintOpts) string {
				return e.TriggerTime.AsTime().In(time.Local).Format(time.RFC3339)
			},
			TrimSize: eventTimeTrimSize,
		},
		{
			FieldName: "DURATION",
			FieldValueFunc: func(e *openv1alpha1resource.Event, opts *table.PrintOpts) string {
				return e.Duration.AsDuration().String()
			},
			TrimSize: eventTimeTrimSize,
		},
	}

	return table.ColumnDefs2Table(fullColumnDefs, p.Delegate, opts)
}
