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

package sentry_utils

import (
	"time"

	"github.com/getsentry/sentry-go"
)

type SentryRunOptions struct {
	RoutineName string
	OnErrorFn   func()
}

// Run wraps a function with Sentry local hub initialization and runs it in a goroutine.
func (o SentryRunOptions) Run(fn func(*sentry.Hub)) {
	localHub := sentry.CurrentHub().Clone()
	go func() {
		defer localHub.Flush(2 * time.Second)
		defer func() {
			if r := recover(); r != nil {
				localHub.Recover(r)

				if o.OnErrorFn != nil {
					o.OnErrorFn()
				}

				panic(r)
			}
		}()

		if o.RoutineName != "" {
			localHub.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("routine_name", o.RoutineName)
			})
		}

		fn(localHub)
	}()
}
