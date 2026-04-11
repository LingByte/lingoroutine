package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import "net/http"

// HeaderTransport a http round tripper
type HeaderTransport struct {
	Header    http.Header
	Set       bool
	transport http.RoundTripper
}

// RoundTrip : The Request's URL and Header fields must be initialized.
func (t *HeaderTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {

	for key, values := range t.Header {
		if t.Set {
			if len(values) > 0 {
				req.Header.Set(key, values[len(values)-1])
			}
		} else {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}

	return t.transport.RoundTrip(req)
}

// NewHeaderTransport return a BasicTransport
func NewHeaderTransport(header http.Header, transport http.RoundTripper, set bool) *HeaderTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &HeaderTransport{
		Header:    header,
		Set:       set,
		transport: transport,
	}
}

// header := http.Header{}

// header.Set("Cookies","xxxx")

// tr := NewHeaderTransport(header, nil)
