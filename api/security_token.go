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
	"time"

	openDssv1alphaconnect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/datastorage/v1alpha1/services/servicesconnect"
	openDssv1alphaservice "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/datastorage/v1alpha1/services"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
)

type SecurityTokenInterface interface {
	GenerateSecurityToken(ctx context.Context, project string) (*openDssv1alphaservice.GenerateSecurityTokenResponse, error)
}

type securityTokenClient struct {
	securityTokenServiceClient openDssv1alphaconnect.SecurityTokenServiceClient
}

func NewSecurityTokenClient(securityTokenServiceClient openDssv1alphaconnect.SecurityTokenServiceClient) SecurityTokenInterface {
	return &securityTokenClient{
		securityTokenServiceClient: securityTokenServiceClient,
	}
}

func (c *securityTokenClient) GenerateSecurityToken(ctx context.Context, project string) (*openDssv1alphaservice.GenerateSecurityTokenResponse, error) {
	req := connect.NewRequest(&openDssv1alphaservice.GenerateSecurityTokenRequest{
		Project: project,
		ExpireDuration: &durationpb.Duration{
			Seconds: int64((7 * 24 * time.Hour).Seconds()),
		},
	})
	res, err := c.securityTokenServiceClient.GenerateSecurityToken(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}
