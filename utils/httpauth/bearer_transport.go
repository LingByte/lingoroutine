package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import "net/http"

// BearerTransport a http round tripper
type BearerTransport struct {
	AccessToken string
	transport   http.RoundTripper
}

// RoundTrip : The Request's URL and Header fields must be initialized.
func (t *BearerTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {

	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	return t.transport.RoundTrip(req)
}

// NewBearerTransport return a BearerTransport
func NewBearerTransport(accessToken string, transport http.RoundTripper) *BearerTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &BearerTransport{
		AccessToken: accessToken,
		transport:   transport,
	}
}
