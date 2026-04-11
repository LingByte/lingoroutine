package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"net"
	"net/http"
	"time"
)

// AuthorizationTransport a http round tripper
type AuthorizationTransport struct {
	Token     string
	transport http.RoundTripper
}

// RoundTrip : The Request's URL and Header fields must be initialized.
func (t *AuthorizationTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	req.Header.Set("Authorization", t.Token)
	return t.transport.RoundTrip(req)
}

// NewAuthorizationTransport return a AuthorizationTransport

func NewAuthorizationTransport(token string, transport http.RoundTripper) *AuthorizationTransport {
	if transport == nil {
		transport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			MaxIdleConnsPerHost:   50,
			MaxIdleConns:          100,
			IdleConnTimeout:       40 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
		}
	}

	return &AuthorizationTransport{
		Token:     token,
		transport: transport,
	}
}
