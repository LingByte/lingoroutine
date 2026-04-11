package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var bearerTransport *BearerTransport

func TestNewBearerTransport(t *testing.T) {
	assert := assert.New(t)
	bearerTransport = NewBearerTransport("ec33ca49ec63b7e28c030172d2cbd72ab6ed5e44", nil)
	assert.NotNil(bearerTransport)
}

func TestBearerTransportRoundTrip(t *testing.T) {
	assert := assert.New(t)
	req, err := http.NewRequest("GET", "http://example.com", nil)
	assert.Nil(err)
	assert.Empty(req.Header.Get("Authorization"), "authorization header key must empty")

	resp, err := bearerTransport.RoundTrip(req)
	assert.Nil(err)
	assert.NotEmpty(req.Header.Get("Authorization"), "authorization header key must not empty")
	assert.NotNil(resp)

	auth := "Bearer ec33ca49ec63b7e28c030172d2cbd72ab6ed5e44"
	assert.Equal(req.Header.Get("Authorization"), auth, "Two authorization header key should be the same.")
}
