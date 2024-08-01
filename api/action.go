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
	"strings"

	openv1alpha1connect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/dataplatform/v1alpha1/services/servicesconnect"
	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	openv1alpha1service "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/services"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/samber/lo"
)

type ActionInterface interface {
	// GetByName gets an action by name.
	GetByName(ctx context.Context, actionName *name.Action) (*openv1alpha1resource.Action, error)

	// ListAllActions lists all actions in the current organization.
	ListAllActions(ctx context.Context, listOpts *ListActionsOptions) ([]*openv1alpha1resource.Action, error)

	// CreateActionRun creates an action run.
	CreateActionRun(ctx context.Context, action *openv1alpha1resource.Action, record *name.Record) error

	// ListAllActionRuns lists all action runs in the current organization.
	ListAllActionRuns(ctx context.Context, listOpts *ListActionRunsOptions) ([]*openv1alpha1resource.ActionRun, error)

	// ActionId2Name converts an action id or name to an action name.
	ActionId2Name(ctx context.Context, actionIdOrName string, projectNameStr *name.Project) (*name.Action, error)
}

type actionClient struct {
	actionServiceClient    openv1alpha1connect.ActionServiceClient
	actionRunServiceClient openv1alpha1connect.ActionRunServiceClient
}

func NewActionClient(
	actionServiceClient openv1alpha1connect.ActionServiceClient,
	actionRunServiceClient openv1alpha1connect.ActionRunServiceClient,
) ActionInterface {
	return &actionClient{
		actionServiceClient:    actionServiceClient,
		actionRunServiceClient: actionRunServiceClient,
	}
}

type ListActionsOptions struct {
	Parent string
}

type ListActionRunsOptions struct {
	Parent      string
	RecordNames []*name.Record
}

func (c *actionClient) GetByName(ctx context.Context, actionName *name.Action) (*openv1alpha1resource.Action, error) {
	req := connect.NewRequest(&openv1alpha1service.GetActionRequest{
		Name: actionName.String(),
	})
	res, err := c.actionServiceClient.GetAction(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get action: %w", err)
	}

	return res.Msg, nil
}

func (c *actionClient) ListAllActions(ctx context.Context, listOpts *ListActionsOptions) ([]*openv1alpha1resource.Action, error) {
	filter := c.filter(listOpts)

	var (
		skip = 0
		ret  []*openv1alpha1resource.Action
	)

	for {
		req := connect.NewRequest(&openv1alpha1service.ListActionsRequest{
			Parent:   listOpts.Parent,
			Filter:   filter,
			Skip:     int32(skip),
			PageSize: int32(constants.MaxPageSize),
		})
		res, err := c.actionServiceClient.ListActions(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list actions: %w", err)
		}

		ret = append(ret, res.Msg.Actions...)
		if len(res.Msg.Actions) < constants.MaxPageSize {
			break
		}
		skip += constants.MaxPageSize
	}

	return ret, nil
}

func (c *actionClient) filter(opt *ListActionsOptions) string {
	return ""
}

func (c *actionClient) CreateActionRun(ctx context.Context, action *openv1alpha1resource.Action, record *name.Record) error {
	req := connect.NewRequest(&openv1alpha1service.CreateActionRunRequest{
		Parent: record.Project().String(),
		ActionRun: &openv1alpha1resource.ActionRun{
			Action: action,
			Records: []*openv1alpha1resource.Record{
				{
					Name: record.String(),
				},
			},
		},
	})
	_, err := c.actionRunServiceClient.CreateActionRun(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create action run: %w", err)
	}

	return nil
}

func (c *actionClient) ListAllActionRuns(ctx context.Context, listOpts *ListActionRunsOptions) ([]*openv1alpha1resource.ActionRun, error) {
	filter := c.filterRun(listOpts)

	var (
		skip = 0
		ret  []*openv1alpha1resource.ActionRun
	)

	for {
		req := connect.NewRequest(&openv1alpha1service.ListActionRunsRequest{
			Parent:   listOpts.Parent,
			Filter:   filter,
			Skip:     int32(skip),
			PageSize: int32(constants.MaxPageSize),
		})
		res, err := c.actionRunServiceClient.ListActionRuns(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list action runs: %w", err)
		}

		ret = append(ret, res.Msg.ActionRuns...)
		if len(res.Msg.ActionRuns) < constants.MaxPageSize {
			break
		}
		skip += constants.MaxPageSize
	}

	return ret, nil
}

func (c *actionClient) filterRun(opts *ListActionRunsOptions) string {
	var filters []string
	if opts.RecordNames != nil {
		filters = append(
			filters,
			fmt.Sprintf(
				"match.records==[%s]",
				strings.Join(lo.Map(opts.RecordNames, func(r *name.Record, _ int) string { return "\"" + r.String() + "\"" }), ","),
			),
		)
	}
	return strings.Join(filters, " && ")
}

func (c *actionClient) ActionId2Name(ctx context.Context, actionIdOrName string, projectName *name.Project) (*name.Action, error) {
	actionName, err := name.NewAction(actionIdOrName)
	if err == nil {
		return actionName, nil
	}

	if !name.IsUUID(actionIdOrName) {
		return nil, fmt.Errorf("invalid action id or name: %s", actionIdOrName)
	}

	// Try fetching assuming it's a project action
	if act, err := c.GetByName(ctx, &name.Action{
		ProjectID: projectName.ProjectID,
		ID:        actionIdOrName,
	}); err == nil {
		return name.NewAction(act.Name)
	}

	if act, err := c.GetByName(ctx, &name.Action{
		ID: actionIdOrName,
	}); err == nil {
		return name.NewAction(act.Name)
	}

	return nil, fmt.Errorf("failed to convert action id to name: %s", actionIdOrName)
}
