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
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"google.golang.org/genproto/protobuf/field_mask"
)

type RecordInterface interface {
	// Get gets a record by name.
	Get(ctx context.Context, recordName *name.Record) (*openv1alpha1resource.Record, error)

	// Create creates a record.
	Create(ctx context.Context, parent *name.Project, title string, deviceNameStr string, description string, labelDisplayNames []*openv1alpha1resource.Label) (*openv1alpha1resource.Record, error)

	// Copy copies a record to target project.
	Copy(ctx context.Context, recordName *name.Record, targetProjectName *name.Project) (*openv1alpha1resource.Record, error)

	// CopyFiles copies files from src record to dst record.
	CopyFiles(ctx context.Context, srcRecordName *name.Record, dstRecordName *name.Record, files []*openv1alpha1resource.File) error

	// ListAllFiles lists all files in a record.
	ListAllFiles(ctx context.Context, recordName *name.Record) ([]*openv1alpha1resource.File, error)

	// Delete deletes a record by name.
	Delete(ctx context.Context, recordName *name.Record) error

	// Update updates a record.
	Update(ctx context.Context, recordName *name.Record, title string, description string, labels []*openv1alpha1resource.Label, fieldMask []string) error

	//ListAllEvents lists all events in a record.
	ListAllEvents(ctx context.Context, recordName *name.Record) ([]*openv1alpha1resource.Event, error)

	// ListAll lists all records in a project.
	ListAll(ctx context.Context, options *ListRecordsOptions) ([]*openv1alpha1resource.Record, error)

	// GenerateRecordThumbnailUploadUrl generates a pre-signed URL for uploading a record thumbnail.
	GenerateRecordThumbnailUploadUrl(ctx context.Context, recordName *name.Record) (string, error)

	// RecordId2Name converts a record id or name to a record name.
	RecordId2Name(ctx context.Context, recordIdOrName string, projectNameStr *name.Project) (*name.Record, error)
}

type ListRecordsOptions struct {
	Project        *name.Project
	Titles         []string
	IncludeArchive bool
}

type recordClient struct {
	recordServiceClient openv1alpha1connect.RecordServiceClient
	fileServiceClient   openv1alpha1connect.FileServiceClient
}

func NewRecordClient(recordServiceClient openv1alpha1connect.RecordServiceClient, fileServiceClient openv1alpha1connect.FileServiceClient) RecordInterface {
	return &recordClient{
		recordServiceClient: recordServiceClient,
		fileServiceClient:   fileServiceClient,
	}
}

func (c *recordClient) Get(ctx context.Context, recordName *name.Record) (*openv1alpha1resource.Record, error) {
	getRecordReq := connect.NewRequest(&openv1alpha1service.GetRecordRequest{
		Name: recordName.String(),
	})
	getRecordRes, err := c.recordServiceClient.GetRecord(ctx, getRecordReq)
	if err != nil {
		return nil, err
	}
	return getRecordRes.Msg, nil
}

func (c *recordClient) Create(ctx context.Context, parent *name.Project, title string, deviceNameStr string, description string, labels []*openv1alpha1resource.Label) (*openv1alpha1resource.Record, error) {
	var (
		device *openv1alpha1resource.Device = nil
	)
	if len(deviceNameStr) > 0 {
		device = &openv1alpha1resource.Device{Name: deviceNameStr}
	}

	req := connect.NewRequest(&openv1alpha1service.CreateRecordRequest{
		Parent: parent.String(),
		Record: &openv1alpha1resource.Record{
			Title:       title,
			Description: description,
			Device:      device,
			Labels:      labels,
		},
	})
	resp, err := c.recordServiceClient.CreateRecord(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, err
}

func (c *recordClient) Copy(ctx context.Context, recordName *name.Record, targetProjectName *name.Project) (*openv1alpha1resource.Record, error) {
	req := connect.NewRequest(&openv1alpha1service.CopyRecordsRequest{
		Parent:      recordName.Project().String(),
		Destination: targetProjectName.String(),
		Records:     []string{recordName.String()},
	})
	resp, err := c.recordServiceClient.CopyRecords(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Msg.Records) != 1 {
		return nil, errors.Errorf("unexpected number of records in response: %d", len(resp.Msg.Records))
	}
	return resp.Msg.Records[0], nil
}

func (c *recordClient) CopyFiles(ctx context.Context, srcRecordName *name.Record, dstRecordName *name.Record, files []*openv1alpha1resource.File) error {
	copyPairs := lo.Map(files, func(file *openv1alpha1resource.File, _ int) *openv1alpha1service.CopyFilesRequest_CopyPair {
		srcFileName, _ := name.NewFile(file.Name)
		return &openv1alpha1service.CopyFilesRequest_CopyPair{
			SrcFile: srcFileName.Filename,
			DstFile: srcFileName.Filename,
		}
	})

	req := connect.NewRequest(&openv1alpha1service.CopyFilesRequest{
		Parent:      srcRecordName.String(),
		Destination: dstRecordName.String(),
		CopyPairs:   copyPairs,
	})
	_, err := c.fileServiceClient.CopyFiles(ctx, req)
	return err
}

func (c *recordClient) ListAllFiles(ctx context.Context, recordName *name.Record) ([]*openv1alpha1resource.File, error) {
	var (
		skip = 0
		ret  []*openv1alpha1resource.File
	)

	filter := "recursive=\"true\""

	for {
		req := connect.NewRequest(&openv1alpha1service.ListFilesRequest{
			Parent:   recordName.String(),
			PageSize: constants.MaxPageSize,
			Skip:     int32(skip),
			Filter:   filter,
		})
		res, err := c.fileServiceClient.ListFiles(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list files at skip %d: %w", skip, err)
		}
		if len(res.Msg.Files) == 0 {
			break
		}
		ret = append(ret, res.Msg.Files...)
		skip += constants.MaxPageSize
	}

	return lo.Filter(ret, func(file *openv1alpha1resource.File, _ int) bool {
		return !strings.HasSuffix(file.Filename, "/")
	}), nil
}

func (c *recordClient) Delete(ctx context.Context, recordName *name.Record) error {
	deleteRecordReq := connect.NewRequest(&openv1alpha1service.DeleteRecordRequest{
		Name: recordName.String(),
	})
	_, err := c.recordServiceClient.DeleteRecord(ctx, deleteRecordReq)
	return err
}

func (c *recordClient) Update(ctx context.Context, recordName *name.Record, title string, description string, labels []*openv1alpha1resource.Label, fieldMask []string) error {
	req := connect.NewRequest(&openv1alpha1service.UpdateRecordRequest{
		Record: &openv1alpha1resource.Record{
			Name:        recordName.String(),
			Title:       title,
			Description: description,
			Labels:      labels,
		},
		UpdateMask: &field_mask.FieldMask{
			Paths: fieldMask,
		},
	})
	_, err := c.recordServiceClient.UpdateRecord(ctx, req)
	return err
}

func (c *recordClient) ListAllEvents(ctx context.Context, recordName *name.Record) ([]*openv1alpha1resource.Event, error) {
	var (
		skip = 0
		ret  []*openv1alpha1resource.Event
	)

	for {
		req := connect.NewRequest(&openv1alpha1service.ListRecordEventsRequest{
			Parent:   recordName.String(),
			PageSize: constants.MaxPageSize,
			Skip:     int32(skip),
			Filter:   "",
		})
		res, err := c.recordServiceClient.ListRecordEvents(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list events at skip %d: %w", skip, err)
		}
		if len(res.Msg.Events) == 0 {
			break
		}
		ret = append(ret, res.Msg.Events...)
		skip += constants.MaxPageSize
	}

	return ret, nil
}

func (c *recordClient) ListAll(ctx context.Context, options *ListRecordsOptions) ([]*openv1alpha1resource.Record, error) {
	if options.Project.ProjectID == "" {
		return nil, errors.Errorf("invalid project: %s", options.Project)
	}

	filter := c.filter(options)

	var (
		skip = 0
		ret  []*openv1alpha1resource.Record
	)

	for {
		req := connect.NewRequest(&openv1alpha1service.ListRecordsRequest{
			Parent:   options.Project.String(),
			PageSize: constants.MaxPageSize,
			Skip:     int32(skip),
			Filter:   filter,
		})
		res, err := c.recordServiceClient.ListRecords(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list records at skip %d: %w", skip, err)
		}
		if len(res.Msg.Records) == 0 {
			break
		}
		ret = append(ret, res.Msg.Records...)
		skip += constants.MaxPageSize
	}

	return ret, nil
}

func (c *recordClient) filter(opts *ListRecordsOptions) string {
	var filters []string
	if !opts.IncludeArchive {
		filters = append(filters, "is_archived=false")
	}
	if len(opts.Titles) > 0 {
		filters = append(filters, "("+strings.Join(
			lo.Map(opts.Titles, func(title string, _ int) string { return fmt.Sprintf(`title:"%s"`, title) }),
			` OR `,
		)+")")
	}
	return strings.Join(filters, " AND ")
}

func (c *recordClient) GenerateRecordThumbnailUploadUrl(ctx context.Context, recordName *name.Record) (string, error) {
	req := connect.NewRequest(&openv1alpha1service.GenerateRecordThumbnailUploadUrlRequest{
		Record: recordName.String(),
	})
	resp, err := c.recordServiceClient.GenerateRecordThumbnailUploadUrl(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Msg.PreSignedUri, nil
}

func (c *recordClient) RecordId2Name(ctx context.Context, recordIdOrName string, projectName *name.Project) (*name.Record, error) {
	recordName, err := name.NewRecord(recordIdOrName)
	if err == nil {
		return recordName, nil
	}

	recordName = &name.Record{
		ProjectID: projectName.ProjectID,
		RecordID:  recordIdOrName,
	}

	if _, err := c.Get(ctx, recordName); err != nil {
		return nil, errors.Wrapf(err, "unable to get record: %s", recordName.String())
	}

	return recordName, nil
}
