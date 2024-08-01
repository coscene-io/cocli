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
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/http2"
)

// NewConnectClient creates a new http client for connecting to the given endpoint.
func NewConnectClient(endpoint string) (*http.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "https":
		return newSecureClient(), nil
	case "http":
		return newInsecureClient(), nil
	default:
		return nil, errors.Errorf("unsupported protocol: %s", u.Scheme)
	}
}

func newSecureClient() *http.Client {
	return http.DefaultClient
}

func newInsecureClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(context context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				// If you're also using this client for non-h2c traffic, you may want
				// to delegate to tls.Dial if the network isn't TCP or the addr isn't
				// in an allowlist.
				return net.Dial(network, addr)
			},
			PingTimeout:      3 * time.Second,
			ReadIdleTimeout:  3 * time.Second,
			WriteByteTimeout: 3 * time.Second,
		},
	}
}
