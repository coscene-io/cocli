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
	"strconv"

	openv1alpha1connect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/dataplatform/v1alpha1/services/servicesconnect"
	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/pkg/errors"
)

type LabelInterface interface {
	// GetByDisplayNameOrCreate gets a label by display name, creates it if not found.
	GetByDisplayNameOrCreate(ctx context.Context, displayName string, projectName *name.Project) (*openv1alpha1resource.Label, error)
}

type labelClient struct {
	labelServiceClient openv1alpha1connect.LabelServiceClient
}

func NewLabelClient(labelServiceClient openv1alpha1connect.LabelServiceClient) LabelInterface {
	return &labelClient{
		labelServiceClient: labelServiceClient,
	}
}

func (c *labelClient) GetByDisplayNameOrCreate(ctx context.Context, displayName string, project *name.Project) (*openv1alpha1resource.Label, error) {
	listLabelsReq := connect.NewRequest(&openv1alpha1service.ListLabelsRequest{
		Parent:   project.String(),
		PageSize: 10,
		Skip:     0,
		Filter:   fmt.Sprintf("display_name=%s", strconv.Quote(displayName)),
	})
	listLabelsRes, err := c.labelServiceClient.ListLabels(ctx, listLabelsReq)
	if err == nil && len(listLabelsRes.Msg.Labels) == 1 && listLabelsRes.Msg.TotalSize == 1 {
		return listLabelsRes.Msg.Labels[0], nil
	}

	// Failed to get label by name, might not exist create it.
	createLabelReq := connect.NewRequest(&openv1alpha1service.CreateLabelRequest{
		Parent: project.String(),
		Label: &openv1alpha1resource.Label{
			DisplayName: displayName,
		},
	})
	createLabelRes, err := c.labelServiceClient.CreateLabel(context.TODO(), createLabelReq)
	if err != nil {
		return nil, errors.Wrapf(err, "create label %s failed", displayName)
	}
	return createLabelRes.Msg, nil
}
