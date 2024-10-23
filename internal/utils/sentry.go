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

package utils

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
)

// SentryRunOptions is the options for running a function with Sentry local hub initialization.
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

// sentryHook is a logrus hook for sending log messages to Sentry.
type sentryHook struct {
	levels []log.Level
}

// NewSentryHook initializes a new sentryHook with specified log levels.
func NewSentryHook() log.Hook {
	return &sentryHook{levels: []log.Level{log.FatalLevel, log.PanicLevel}}
}

// Levels returns the log levels that trigger the Sentry hook.
func (hook *sentryHook) Levels() []log.Level {
	return hook.levels
}

// Fire sends the log entry to Sentry.
func (hook *sentryHook) Fire(entry *log.Entry) error {
	// Prepare the message and the level to send to Sentry.
	message := fmt.Sprintf("%s: %s", entry.Level.String(), entry.Message)
	if entry.Level == log.FatalLevel || entry.Level == log.PanicLevel {
		// Capture fatal or panic messages as Sentry fatal events
		sentry.CaptureMessage(message)
		// Ensure Sentry sends the message before the program exits
		sentry.Flush(2 * time.Second)
	} else {
		// Capture error messages or other levels as regular events
		sentry.CaptureMessage(message)
	}
	return nil
}
