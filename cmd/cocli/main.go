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

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"

	"github.com/coscene-io/cocli/pkg/cmd"
	log "github.com/sirupsen/logrus"
)

const (
	profilePath = "/tmp/cpu_profile"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
	})

	if isPProfEnabled() {
		cpuf, err := os.Create(profilePath)
		if err != nil {
			log.Fatal(err)
		}
		defer cpuf.Close()

		runtime.SetCPUProfileRate(getProfileHZ())
		err = pprof.StartCPUProfile(cpuf)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Dump cpu profile file into %s.", profilePath)
		defer pprof.StopCPUProfile()
	}

	if err := cmd.NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func isPProfEnabled() (enable bool) {
	for _, arg := range os.Args {
		if arg == "--pprof" {
			enable = true
			break
		}
	}

	return
}

func getProfileHZ() int {
	profileRate := 1000
	if s, err := strconv.Atoi(os.Getenv("PROFILE_RATE")); err == nil {
		profileRate = s
	}
	return profileRate
}
