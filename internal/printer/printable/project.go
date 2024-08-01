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
	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/internal/printer/table"
	"google.golang.org/protobuf/proto"
)

const (
	projectIdTrimSize   = 36
	projectSlugTrimSize = 30
)

type Project struct {
	Delegate []*openv1alpha1resource.Project
}

func NewProject(projects []*openv1alpha1resource.Project) *Project {
	return &Project{
		Delegate: projects,
	}
}

func (p *Project) ToProtoMessage() proto.Message {
	return &openv1alpha1service.ListProjectsResponse{
		Projects:  p.Delegate,
		TotalSize: int64(len(p.Delegate)),
	}
}

func (p *Project) ToTable(opts *table.PrintOpts) table.Table {
	fullColumnDefs := []table.ColumnDefinitionFull[*openv1alpha1resource.Project]{
		{
			FieldNameFunc: func(opts *table.PrintOpts) string {
				if opts.Verbose {
					return "RESOURCE NAME"
				}
				return "ID"
			},
			FieldValueFunc: func(p *openv1alpha1resource.Project, opts *table.PrintOpts) string {
				if opts.Verbose {
					return p.Name
				}
				projectName, _ := name.NewProject(p.Name)
				return projectName.ProjectID
			},
			TrimSize: projectIdTrimSize,
		},
		{
			FieldName: "SLUG",
			FieldValueFunc: func(p *openv1alpha1resource.Project, opts *table.PrintOpts) string {
				return p.Slug
			},
			TrimSize: projectSlugTrimSize,
		},
	}

	return table.ColumnDefs2Table(fullColumnDefs, p.Delegate, opts)
}
