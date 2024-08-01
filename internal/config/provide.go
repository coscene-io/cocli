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
	"os"
	"strings"

	"dario.cat/mergo"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
)

// Provider is an interface for providing the configuration
type Provider interface {
	GetProfileManager() (*ProfileManager, error)
	Persist(pm *ProfileManager) error
}

// globalConfig implements Provider
type globalConfig struct {
	path           string     `koanf:"-"`
	CurrentProfile string     `koanf:"current-profile"`
	Profiles       []*Profile `koanf:"profiles"`
}

func Provide(path string) Provider {
	return &globalConfig{path: path}
}

// GetProfileManager loads the profile manager from the config file
// Note that loading from config have higher priority and that loading from env have the following format:
// COS_ENDPOINT,
// COS_TOKEN,
// COS_PROJECT
// Note that loading profile from env will change the config file
func (cfg *globalConfig) GetProfileManager() (*ProfileManager, error) {
	if err := cfg.loadYaml("current-profile", &cfg.CurrentProfile); err != nil {
		return nil, errors.Wrapf(err, "unable to load current-profile from %s", cfg.path)
	}
	if err := cfg.loadYaml("profiles", &cfg.Profiles); err != nil {
		return nil, errors.Wrapf(err, "unable to load profiles from %s", cfg.path)
	}

	pm := new(ProfileManager)
	pm.CurrentProfile = cfg.CurrentProfile
	pm.Profiles = cfg.Profiles

	if err := pm.Validate(); err != nil {
		return nil, errors.Wrapf(err, "profile validation failed")
	} else if !pm.IsEmpty() {
		return pm, nil
	}

	// Config profile empty, loading from env
	envLoadedProfile := &Profile{Name: "ENV_LOADED_PROFILE"}
	if err := cfg.loadEnv("", envLoadedProfile); err != nil {
		return nil, errors.Wrapf(err, "unable to load profile from env")
	}
	if envLoadedProfile.EndPoint == "" || envLoadedProfile.Token == "" || envLoadedProfile.ProjectSlug == "" {
		return pm, nil
	}
	pm = new(ProfileManager)
	pm.CurrentProfile = envLoadedProfile.Name
	pm.Profiles = []*Profile{envLoadedProfile}

	if err := pm.Validate(); err != nil {
		return nil, errors.Wrapf(err, "profile validation failed")
	}

	return pm, nil
}

// Persist saves the profile manager to the config file
func (cfg *globalConfig) Persist(pm *ProfileManager) error {
	cfg.CurrentProfile = pm.CurrentProfile
	cfg.Profiles = pm.Profiles
	return cfg.persist()
}

func (cfg *globalConfig) loadYaml(path string, any interface{}) error {
	k := koanf.New(".")
	if err := k.Load(file.Provider(cfg.path), yaml.Parser()); err != nil {
		return errors.Wrapf(err, "unable to load config from yaml %s", cfg.path)
	}

	if err := k.Unmarshal(path, any); err != nil {
		return errors.Wrapf(err, "unable to unmarshal config from %s", cfg.path)
	}

	return nil
}

// persist saves the current config as an update to the original config file
func (cfg *globalConfig) persist() error {
	// Load original config
	originalConfig := &globalConfig{path: cfg.path}
	err := cfg.loadYaml("", originalConfig)
	if err != nil {
		return errors.Wrapf(err, "unable to load config from %s", cfg.path)
	}

	// Update original with current
	err = mergo.Merge(originalConfig, cfg, mergo.WithOverride)
	if err != nil {
		return errors.Wrapf(err, "unable to merge config")
	}

	k := koanf.New(".")

	// load updated originalConfig to k
	err = k.Load(structs.Provider(originalConfig, "koanf"), nil)
	if err != nil {
		return errors.Wrapf(err, "unable to load config to k from original config")
	}
	// marshal k to yamlStr
	yamlStr, err := k.Marshal(yaml.Parser())
	if err != nil {
		return errors.Wrapf(err, "unable to marshal k to yaml")
	}

	// write yamlStr to globalConfig.path
	err = os.WriteFile(originalConfig.path, yamlStr, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to write yaml to %s", originalConfig.path)
	}
	return nil
}

// loadEnv loads the config from environment variables
func (cfg *globalConfig) loadEnv(path string, any interface{}) error {
	k := koanf.New(".")
	if err := k.Load(
		env.Provider(
			"COS",
			"_",
			func(s string) string {
				return strings.ToLower(strings.TrimPrefix(s, "COS_"))
			},
		),
		nil,
	); err != nil {
		return errors.Wrapf(err, "load config from env")
	}

	if err := k.Unmarshal(path, any); err != nil {
		return errors.Wrap(err, "unmarshal env")
	}
	return nil
}
