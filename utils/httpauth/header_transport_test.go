package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var headertransport *HeaderTransport

func TestNewHeaderTransport(t *testing.T) {
	assert := assert.New(t)
	header := http.Header{}
	header.Add("Cookie", "Hm_lvt_889f42ef09762f0358f1c1bb6feaf95e=1484187653")
	headertransport := NewHeaderTransport(header, nil, true)
	assert.NotNil(headertransport)

	req, err := http.NewRequest("GET", "http://example.com", nil)
	resp, err := headertransport.RoundTrip(req)
	assert.Nil(err)
	assert.NotEmpty(req.Header.Get("Cookie"), "cookie header key must not empty")
	assert.NotNil(resp)

	cookie := "Hm_lvt_889f42ef09762f0358f1c1bb6feaf95e=1484187653"
	assert.Equal(req.Header.Get("Cookie"), cookie, "Two cookie header key should be the same.")

}
