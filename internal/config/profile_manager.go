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

	"dario.cat/mergo"
	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/internal/name"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ProfileManager represents a profile manager in the configuration file.
type ProfileManager struct {
	CurrentProfile string     `koanf:"current-profile"`
	Profiles       []*Profile `koanf:"profiles"`
}

// Validate each profile and ensure that all profiles have different names.
// Note that empty profile manager is valid.
func (pm *ProfileManager) Validate() error {
	profileSet := mapset.NewSet[string]()
	for _, profile := range pm.Profiles {
		if profileSet.Contains(profile.Name) {
			return errors.Errorf("profile %s already exists", profile.Name)
		}
		profileSet.Add(profile.Name)

		if err := profile.Validate(); err != nil {
			return errors.Wrap(err, "single profile validation failed")
		}
	}

	if profileSet.Cardinality() > 0 && !profileSet.Contains(pm.CurrentProfile) {
		return errors.Errorf("current profile %s not found", pm.CurrentProfile)
	}

	if profileSet.Cardinality() == 0 && pm.CurrentProfile != "" {
		return errors.Errorf("current profile %s not found", pm.CurrentProfile)
	}

	return nil
}

// IsEmpty check if the profile manager is empty
func (pm *ProfileManager) IsEmpty() bool {
	return len(pm.Profiles) == 0
}

// CheckAuth check if the current login profile is authenticated
func (pm *ProfileManager) CheckAuth() bool {
	return pm.GetCurrentProfile().CheckAuth()
}

// Auth authenticate the current login profile and modify the profile manager
func (pm *ProfileManager) Auth() error {
	return pm.GetCurrentProfile().Auth()
}

// GetRecordUrl gets the url of the record in the corresponding coScene website.
func (pm *ProfileManager) GetRecordUrl(recordName *name.Record) (string, error) {
	return pm.GetCurrentProfile().GetRecordUrl(recordName)
}

// GetBaseUrl returns the base url of the corresponding coScene website.
func (pm *ProfileManager) GetBaseUrl() string {
	return pm.GetCurrentProfile().GetBaseUrl()
}

// ProjectName return project name used slug parameter preferred to current profile.
func (pm *ProfileManager) ProjectName(ctx context.Context, slug string) (*name.Project, error) {
	profile := pm.GetCurrentProfile()
	if slug != "" {
		proj, err := profile.ProjectCli().Name(ctx, slug)
		if err != nil {
			return nil, errors.Wrapf(err, "name project slug: %v", slug)
		}
		return proj, err
	}

	proj, err := name.NewProject(profile.ProjectName)
	if err != nil {
		return nil, errors.Wrapf(err, "new project name: %v", profile.ProjectName)
	}
	return proj, nil
}

// RecordCli return record client of current profile.
func (pm *ProfileManager) RecordCli() api.RecordInterface {
	return pm.GetCurrentProfile().RecordCli()
}

// ProjectCli return project client of current profile.
func (pm *ProfileManager) ProjectCli() api.ProjectInterface {
	return pm.GetCurrentProfile().ProjectCli()
}

// LabelCli return label client of current profile.
func (pm *ProfileManager) LabelCli() api.LabelInterface {
	return pm.GetCurrentProfile().LabelCli()
}

// UserCli return user client of current profile.
func (pm *ProfileManager) UserCli() api.UserInterface {
	return pm.GetCurrentProfile().UserCli()
}

// FileCli return file client of current profile.
func (pm *ProfileManager) FileCli() api.FileInterface {
	return pm.GetCurrentProfile().FileCli()
}

// ActionCli return action client of current profile.
func (pm *ProfileManager) ActionCli() api.ActionInterface {
	return pm.GetCurrentProfile().ActionCli()
}

// SecurityTokenCli return security token client of current profile.
func (pm *ProfileManager) SecurityTokenCli() api.SecurityTokenInterface {
	return pm.GetCurrentProfile().SecurityTokenCli()
}

// GetCurrentProfile return current profile of profile manager.
func (pm *ProfileManager) GetCurrentProfile() *Profile {
	for i, profile := range pm.Profiles {
		if profile.Name == pm.CurrentProfile {
			return pm.Profiles[i]
		}
	}

	// This is a fallback, it should never be reached
	return nil
}

// GetProfiles return all profiles of profile manager.
func (pm *ProfileManager) GetProfiles() []*Profile {
	return lo.Map(pm.Profiles, func(p *Profile, _ int) *Profile { return p })
}

// AddProfile add a new profile to the profile manager.
func (pm *ProfileManager) AddProfile(profile *Profile) error {
	if err := profile.Validate(); err != nil {
		return errors.Wrap(err, "added profile validation failed")
	}
	if err := profile.Auth(); err != nil {
		return errors.Wrap(err, "added profile auth failed")
	}

	pm.Profiles = append(pm.Profiles, profile)
	if pm.CurrentProfile == "" {
		pm.CurrentProfile = profile.Name
	}

	if err := pm.Validate(); err != nil {
		return errors.Wrap(err, "profile manager validation failed")
	}

	return nil
}

// SetProfile set a profile to the profile manager.
func (pm *ProfileManager) SetProfile(profile *Profile) error {
	if len(pm.Profiles) == 0 {
		pm.Profiles = append(pm.Profiles, profile)
		pm.CurrentProfile = profile.Name
	} else {
		curProfile := pm.GetCurrentProfile()
		err := mergo.Merge(curProfile, profile, mergo.WithOverride)
		if err != nil {
			return err
		}
		pm.CurrentProfile = curProfile.Name
	}

	if err := pm.GetCurrentProfile().Validate(); err != nil {
		return errors.Wrap(err, "single profile validation failed")
	}
	// reset org and project name to re-fetch
	pm.GetCurrentProfile().Org = ""
	pm.GetCurrentProfile().ProjectName = ""
	if err := pm.Auth(); err != nil {
		return errors.Wrap(err, "profile auth failed")
	}

	if err := pm.Validate(); err != nil {
		return errors.Wrap(err, "profile manager validation failed")
	}
	return nil
}

// DeleteProfile delete a profile from the profile manager.
func (pm *ProfileManager) DeleteProfile(name string) error {
	for i, profile := range pm.Profiles {
		if profile.Name == name {
			pm.Profiles = append(pm.Profiles[:i], pm.Profiles[i+1:]...)

			if pm.CurrentProfile == name {
				pm.CurrentProfile = pm.Profiles[0].Name
			}

			return nil
		}
	}
	return errors.Errorf("profile %s not found", name)
}

// SwitchProfile switch the current profile to the specified profile.
func (pm *ProfileManager) SwitchProfile(name string) error {
	for _, c := range pm.Profiles {
		if c.Name == name {
			pm.CurrentProfile = name
			if err := pm.Validate(); err != nil {
				return errors.Wrap(err, "single profile validation failed")
			}
			if err := pm.Auth(); err != nil {
				return errors.Wrap(err, "profile fetch org and project failed")
			}
			return nil
		}
	}
	return errors.Errorf("Invalid profile name %s", name)
}
