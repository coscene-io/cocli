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
	"time"

	"github.com/coscene-io/cocli/pkg/cmd"
	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: true,
	})

	err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://b3bcd9e4d101f927b5f1f7ac67d9b115@sentry.coscene.site/23",
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for tracing.
		// We recommend adjusting this value in production,
		TracesSampleRate: 1.0,
		AttachStacktrace: true,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	// Flush buffered events before the program terminates.
	defer sentry.Flush(2 * time.Second)

	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			panic(r)
		}
	}()

	if err := cmd.NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
