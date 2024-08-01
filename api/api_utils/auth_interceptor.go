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
	"encoding/base64"
	"strings"

	"connectrpc.com/connect"
)

// authInterceptor implements connect.Interceptor.
type authInterceptor struct {
	Token string
}

// AuthInterceptor returns an interceptor that adds the given access token to the request headers.
func AuthInterceptor(accessToken string) connect.Interceptor {
	transformedToken := ""
	if len(strings.Split(accessToken, ".")) == 3 {
		transformedToken = "Bearer " + accessToken
	} else {
		transformedToken = "Basic " + base64.StdEncoding.EncodeToString([]byte("apikey:"+accessToken))
	}
	return &authInterceptor{
		Token: transformedToken,
	}
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	// Same as previous UnaryInterceptorFunc.
	return func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", i.Token)
		req.Header().Set("x-cos-auth-token", i.Token)
		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		conn := next(ctx, spec)

		conn.RequestHeader().Set("Authorization", i.Token)
		conn.RequestHeader().Set("x-cos-auth-token", i.Token)
		return conn
	}
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {

		conn.RequestHeader().Set("Authorization", i.Token)
		conn.RequestHeader().Set("x-cos-auth-token", i.Token)
		return next(ctx, conn)
	}
}
