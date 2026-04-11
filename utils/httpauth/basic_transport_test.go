package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var bt *BasicTransport

func TestNewBasicTransport(t *testing.T) {
	assert := assert.New(t)
	bt = NewBasicTransport("username", "password", nil)
	assert.NotNil(bt)
}

func TestRoundTrip(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("GET", "http://example.com", nil)
	assert.Nil(err)
	assert.Empty(req.Header.Get("Authorization"), "authorization header key must empty")

	resp, err := bt.RoundTrip(req)
	assert.Nil(err)
	assert.NotEmpty(req.Header.Get("Authorization"), "authorization header key must not empty")
	assert.NotNil(resp)

	auth := "Basic " + base64.URLEncoding.EncodeToString([]byte("username"+":"+"password"))
	assert.Equal(req.Header.Get("Authorization"), auth, "Two authorization header key should be the same.")
}
