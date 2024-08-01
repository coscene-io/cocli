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

	openv1alpha1connect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/dataplatform/v1alpha1/services/servicesconnect"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/name"
)

type OrganizationInterface interface {
	// Slug get org slug.
	Slug(ctx context.Context, org *name.Organization) (string, error)
}

type organizationClient struct {
	organizationServiceClient openv1alpha1connect.OrganizationServiceClient
}

func NewOrganizationClient(organizationServiceClient openv1alpha1connect.OrganizationServiceClient) OrganizationInterface {
	return &organizationClient{
		organizationServiceClient: organizationServiceClient,
	}
}

func (c *organizationClient) Slug(ctx context.Context, org *name.Organization) (string, error) {
	req := connect.NewRequest(&openv1alpha1service.GetOrganizationRequest{
		Name: org.String(),
	})
	resp, err := c.organizationServiceClient.GetOrganization(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Msg.Slug, nil
}
