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
	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/name"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
)

type UserInterface interface {
	// BatchGetUsers gets users by usernames.
	BatchGetUsers(ctx context.Context, userNameList mapset.Set[name.User]) (map[string]*openv1alpha1resource.User, error)
}

type userClient struct {
	userServiceClient openv1alpha1connect.UserServiceClient
}

func NewUserClient(userServiceClient openv1alpha1connect.UserServiceClient) UserInterface {
	return &userClient{
		userServiceClient: userServiceClient,
	}
}

func (c *userClient) BatchGetUsers(ctx context.Context, userNameSet mapset.Set[name.User]) (map[string]*openv1alpha1resource.User, error) {
	userNameList := userNameSet.ToSlice()
	if len(userNameList) == 0 {
		return map[string]*openv1alpha1resource.User{}, nil
	}
	req := connect.NewRequest(&openv1alpha1service.BatchGetUsersRequest{
		Names: lo.Map(userNameList, func(u name.User, _ int) string {
			return u.String()
		}),
	})
	res, err := c.userServiceClient.BatchGetUsers(ctx, req)
	if err != nil {
		return nil, err
	}

	return lo.Associate(res.Msg.Users, func(u *openv1alpha1resource.User) (string, *openv1alpha1resource.User) {
		return u.Name, u
	}), nil
}
