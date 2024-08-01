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
	"github.com/dustin/go-humanize"
	"google.golang.org/protobuf/proto"
)

const (
	fileFilenameTrimSize = 40
	fileSizeTrimSize     = 15
	fileTimeTrimSize     = len(time.RFC3339)
)

type File struct {
	Delegate []*openv1alpha1resource.File
}

func NewFile(files []*openv1alpha1resource.File) *File {
	return &File{
		Delegate: files,
	}
}

func (p *File) ToProtoMessage() proto.Message {
	return &openv1alpha1service.ListFilesResponse{
		Files:     p.Delegate,
		TotalSize: int64(len(p.Delegate)),
	}
}

func (p *File) ToTable(opts *table.PrintOpts) table.Table {
	fullColumnDefs := []table.ColumnDefinitionFull[*openv1alpha1resource.File]{
		{
			FieldName: "FILENAME",
			FieldValueFunc: func(f *openv1alpha1resource.File, opts *table.PrintOpts) string {
				return f.Filename
			},
			TrimSize: fileFilenameTrimSize,
		},
		{
			FieldName: "SIZE",
			FieldValueFunc: func(f *openv1alpha1resource.File, opts *table.PrintOpts) string {
				return humanize.Bytes(uint64(f.Size))
			},
			TrimSize: fileSizeTrimSize,
		},
		{
			FieldName: "UPDATE TIME",
			FieldValueFunc: func(f *openv1alpha1resource.File, opts *table.PrintOpts) string {
				return f.UpdateTime.AsTime().In(time.Local).Format(time.RFC3339)
			},
			TrimSize: fileTimeTrimSize,
		},
		{
			FieldName: "CREATE TIME",
			FieldValueFunc: func(f *openv1alpha1resource.File, opts *table.PrintOpts) string {
				return f.CreateTime.AsTime().In(time.Local).Format(time.RFC3339)
			},
			TrimSize: fileTimeTrimSize,
		},
	}

	return table.ColumnDefs2Table(fullColumnDefs, p.Delegate, opts)
}
