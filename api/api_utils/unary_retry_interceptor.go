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

package api_utils

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// UnaryRetryInterceptor returns a UnaryInterceptorFunc that retries the request up to retryMax times.
func UnaryRetryInterceptor(retryMax int) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		retryWaitMin := 1 * time.Second
		retryWaitMax := 5 * time.Second

		return func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			var resp connect.AnyResponse
			var err error
			for attempt := 0; attempt <= retryMax; attempt++ {
				resp, err = next(ctx, req)
				if noNeedRetry(err) {
					return resp, nil
				}
				time.Sleep(retryablehttp.DefaultBackoff(retryWaitMin, retryWaitMax, attempt, nil))
			}
			return resp, errors.Wrapf(err, "retry failed after %d attempts", retryMax)
		}
	}
}

// noNeedRetry returns true if the error is not retryable.
// The error is retryable if connect.Error and the error code is UNKNOWN, INTERNAL, UNAVAILABLE, ABORTED.
func noNeedRetry(err error) bool {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		// match the error code UNKNOWN, INTERNAL, UNAVAILABLE, ABORTED
		return !lo.Contains([]connect.Code{connect.CodeUnknown, connect.CodeInternal, connect.CodeUnavailable, connect.CodeAborted}, connErr.Code())
	}
	return true
}
