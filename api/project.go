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

package api

import (
	"context"
	"fmt"

	openv1alpha1connect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/dataplatform/v1alpha1/services/servicesconnect"
	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/internal/name"
)

type ProjectInterface interface {
	// Name gets the project resource name from the project slug.
	Name(ctx context.Context, projectSlug string) (*name.Project, error)

	// Get gets a project by name.
	Get(ctx context.Context, projectName *name.Project) (*openv1alpha1resource.Project, error)

	// ListAllUserProjects lists all projects in the current organization.
	ListAllUserProjects(ctx context.Context, listOpts *ListProjectsOptions) ([]*openv1alpha1resource.Project, error)
}

type ListProjectsOptions struct {
}

type projectClient struct {
	projectServiceClient openv1alpha1connect.ProjectServiceClient
}

func NewProjectClient(projectServiceClient openv1alpha1connect.ProjectServiceClient) ProjectInterface {
	return &projectClient{
		projectServiceClient: projectServiceClient,
	}
}

func (c *projectClient) Name(ctx context.Context, projectSlug string) (*name.Project, error) {
	getProjectReq := connect.NewRequest(&openv1alpha1service.GetProjectRequest{
		Name: fmt.Sprintf("projects/%s", projectSlug),
	})
	getProjectRes, err := c.projectServiceClient.GetProject(ctx, getProjectReq)
	if err != nil {
		return nil, fmt.Errorf("failed to convert project slug: %w", err)
	}
	proj, _ := name.NewProject(getProjectRes.Msg.Name)

	return proj, nil
}

func (c *projectClient) Get(ctx context.Context, projectName *name.Project) (*openv1alpha1resource.Project, error) {
	req := connect.NewRequest(&openv1alpha1service.GetProjectRequest{
		Name: projectName.String(),
	})
	res, err := c.projectServiceClient.GetProject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return res.Msg, nil
}

func (c *projectClient) ListAllUserProjects(ctx context.Context, listOpts *ListProjectsOptions) ([]*openv1alpha1resource.Project, error) {
	filter := c.filter(listOpts)

	var (
		skip = 0
		ret  []*openv1alpha1resource.Project
	)

	for {
		req := connect.NewRequest(&openv1alpha1service.ListProjectsRequest{
			PageSize: constants.MaxPageSize,
			Skip:     int32(skip),
			Filter:   filter,
		})
		res, err := c.projectServiceClient.ListProjects(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects at skip %d: %w", skip, err)
		}
		if len(res.Msg.Projects) == 0 {
			break
		}
		ret = append(ret, res.Msg.Projects...)
		skip += constants.MaxPageSize
	}

	return ret, nil
}

func (c *projectClient) filter(opts *ListProjectsOptions) string {
	return ""
}
