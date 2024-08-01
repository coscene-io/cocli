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
	"github.com/coscene-io/cocli/internal/name"
)

type FileInterface interface {
	// GetFile gets a file by name.
	GetFile(ctx context.Context, fileResourceName string) (*openv1alpha1resource.File, error)

	// GenerateFileUploadUrls generates pre-signed URLs for file uploads.
	GenerateFileUploadUrls(ctx context.Context, recordName *name.Record, files []*openv1alpha1resource.File) (map[string]string, error)

	// GenerateFileDownloadUrl generates a pre-signed URL for file download.
	GenerateFileDownloadUrl(ctx context.Context, fileResourceName string) (string, error)
}

type fileClient struct {
	fileServiceClient openv1alpha1connect.FileServiceClient
}

func NewFileClient(fileServiceClient openv1alpha1connect.FileServiceClient) FileInterface {
	return &fileClient{
		fileServiceClient: fileServiceClient,
	}
}

func (c *fileClient) GetFile(ctx context.Context, fileResourceName string) (*openv1alpha1resource.File, error) {
	req := connect.NewRequest(&openv1alpha1service.GetFileRequest{
		Name: fileResourceName,
	})
	res, err := c.fileServiceClient.GetFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return res.Msg, nil
}

func (c *fileClient) GenerateFileUploadUrls(ctx context.Context, recordName *name.Record, files []*openv1alpha1resource.File) (map[string]string, error) {
	req := connect.NewRequest(&openv1alpha1service.GenerateFileUploadUrlsRequest{
		Parent: recordName.String(),
		Files:  files,
	})
	res, err := c.fileServiceClient.GenerateFileUploadUrls(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate file upload urls: %w", err)
	}
	return res.Msg.PreSignedUrls, nil
}

func (c *fileClient) GenerateFileDownloadUrl(ctx context.Context, fileResourceName string) (string, error) {
	req := connect.NewRequest(&openv1alpha1service.GenerateFileDownloadURLRequest{
		File: &openv1alpha1resource.File{
			Name: fileResourceName,
		},
	})
	res, err := c.fileServiceClient.GenerateFileDownloadURL(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate file download url: %w", err)
	}
	return res.Msg.PreSignedUrl, nil
}
