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

package config

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	openv1alpha1connect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/dataplatform/v1alpha1/services/servicesconnect"
	openDssv1alphaconnect "buf.build/gen/go/coscene-io/coscene-openapi/connectrpc/go/coscene/openapi/datastorage/v1alpha1/services/servicesconnect"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/api/api_utils"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/pkg/errors"
)

// Profile represents a profile in the configuration file.
// Note that if Org is set, then Token is authorized
// If ProjectName is set, then ProjectSlug is authorized and validated
type Profile struct {
	Name             string `koanf:"name"`
	EndPoint         string `koanf:"endpoint"`
	Token            string `koanf:"token"`
	Org              string `koanf:"org"`
	ProjectSlug      string `koanf:"project"`
	ProjectName      string `koanf:"project-name"`
	cliOnce          sync.Once
	orgcli           api.OrganizationInterface
	projcli          api.ProjectInterface
	rcdcli           api.RecordInterface
	lblcli           api.LabelInterface
	usercli          api.UserInterface
	filecli          api.FileInterface
	actioncli        api.ActionInterface
	securitytokencli api.SecurityTokenInterface
}

func (p *Profile) StringWithOpts(withStar bool, verbose bool) string {
	star := ""
	if withStar {
		star = " (*)"
	}
	if !verbose {
		return fmt.Sprintf(
			"%s%s",
			p.Name, star)

	}
	return fmt.Sprintf(
		"%-20s %s%s\n"+
			"%-20s %s\n"+
			"%-20s %s\n"+
			"%-20s %s\n",
		"Profile Name:", p.Name, star,
		"Endpoint:", p.EndPoint,
		"Organization:", p.Org,
		"Default Project:", p.ProjectSlug)
}

func (p *Profile) String() string {
	return p.StringWithOpts(false, true)
}

// Validate checks if the profile has all the required fields set.
func (p *Profile) Validate() error {
	if p.Name == "" {
		return errors.Errorf("profile name cannot be empty")
	}
	if !strings.HasPrefix(p.EndPoint, "https://openapi.") {
		return errors.Errorf("profile %s's endpoint should start with https://openapi.", p.Name)
	}
	if p.Token == "" {
		return errors.Errorf("profile %s's token cannot be empty", p.Name)
	}
	if p.ProjectSlug == "" {
		return errors.Errorf("profile %s's project cannot be empty", p.Name)
	}
	return nil
}

// CheckAuth checks if the profile has the org and project name set.
func (p *Profile) CheckAuth() bool {
	return p.Org != "" && p.ProjectName != ""
}

// Auth fetches the org and project name from the server if they are not set.
func (p *Profile) Auth() error {
	if p == nil {
		return nil
	}
	if p.Org == "" {
		orgName, _ := name.NewOrganization(constants.CurrentOrgNameStr)
		orgSlug, err := p.OrgCli().Slug(context.TODO(), orgName)
		if err != nil {
			return errors.Wrap(err, "unable to get org slug")
		}
		p.Org = orgSlug
	}

	if p.ProjectName == "" {
		projectName, err := p.ProjectCli().Name(context.TODO(), p.ProjectSlug)
		if err != nil {
			return errors.Wrapf(err, "unable to name slug: %s", p.ProjectSlug)
		}
		p.ProjectName = projectName.String()
	}
	return nil
}

// GetBaseUrl returns the base url of the corresponding coScene website.
func (p *Profile) GetBaseUrl() string {
	baseUrl := ""
	if p.EndPoint == "https://openapi.api.coscene.dev" {
		baseUrl = "https://home.coscene.dev"
	} else {
		baseUrl = "https://" + strings.TrimPrefix(p.EndPoint, "https://openapi.")
	}
	return baseUrl
}

// GetRecordUrl returns the url of the record in the corresponding coScene website.
func (p *Profile) GetRecordUrl(recordName *name.Record) (string, error) {
	proj, err := p.ProjectCli().Get(context.TODO(), recordName.Project())
	if err != nil {
		return "", errors.Wrap(err, "unable to get project")
	}
	recordUrl, err := url.JoinPath(p.GetBaseUrl(), p.Org, proj.Slug, "records", recordName.RecordID)
	if err != nil {
		return "", errors.Wrap(err, "unable to join url path")
	}
	return recordUrl, nil
}

// OrgCli return org api interface used profile.
func (p *Profile) OrgCli() api.OrganizationInterface {
	p.initCli()
	return p.orgcli
}

// ProjectCli return project api interface used profile.
func (p *Profile) ProjectCli() api.ProjectInterface {
	p.initCli()
	return p.projcli
}

// RecordCli return project api interface used profile.
func (p *Profile) RecordCli() api.RecordInterface {
	p.initCli()
	return p.rcdcli
}

// LabelCli return label api interface used profile.
func (p *Profile) LabelCli() api.LabelInterface {
	p.initCli()
	return p.lblcli
}

// UserCli return user api interface used profile.
func (p *Profile) UserCli() api.UserInterface {
	p.initCli()
	return p.usercli
}

// FileCli return file api interface used profile.
func (p *Profile) FileCli() api.FileInterface {
	p.initCli()
	return p.filecli
}

// ActionCli return action api interface used profile.
func (p *Profile) ActionCli() api.ActionInterface {
	p.initCli()
	return p.actioncli
}

// SecurityTokenCli return security token api interface used profile.
func (p *Profile) SecurityTokenCli() api.SecurityTokenInterface {
	p.initCli()
	return p.securitytokencli
}

// initCli initializes the api clients for the profile.
// This function is ensured to be called only once.
func (p *Profile) initCli() {
	p.cliOnce.Do(func() {
		conncli := api_utils.NewConnectClient()
		interceptorsFactory := func() connect.Option {
			return connect.WithInterceptors(api_utils.AuthInterceptor(p.Token), api_utils.UnaryRetryInterceptor(3))
		}

		var (
			actionServiceClient        = openv1alpha1connect.NewActionServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			actionRunServiceClient     = openv1alpha1connect.NewActionRunServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			organizationServiceClient  = openv1alpha1connect.NewOrganizationServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			projectServiceClient       = openv1alpha1connect.NewProjectServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			recordServiceClient        = openv1alpha1connect.NewRecordServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			fileServiceClient          = openv1alpha1connect.NewFileServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			labelServiceClient         = openv1alpha1connect.NewLabelServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			userServiceClient          = openv1alpha1connect.NewUserServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
			securityTokenServiceClient = openDssv1alphaconnect.NewSecurityTokenServiceClient(conncli, p.EndPoint, connect.WithGRPC(), interceptorsFactory())
		)

		p.orgcli = api.NewOrganizationClient(organizationServiceClient)
		p.projcli = api.NewProjectClient(projectServiceClient)
		p.rcdcli = api.NewRecordClient(recordServiceClient, fileServiceClient)
		p.lblcli = api.NewLabelClient(labelServiceClient)
		p.usercli = api.NewUserClient(userServiceClient)
		p.filecli = api.NewFileClient(fileServiceClient)
		p.actioncli = api.NewActionClient(actionServiceClient, actionRunServiceClient)
		p.securitytokencli = api.NewSecurityTokenClient(securityTokenServiceClient)
	})
}
