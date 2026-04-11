package httpauth

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var authorizationTransport *AuthorizationTransport

func TestAuthorizationTransport(t *testing.T) {

	assert := assert.New(t)
	authorizationTransport := NewAuthorizationTransport("um6IEH7mtwnwkGpjImD08JdxlvViuELhI4m", nil)
	assert.NotNil(authorizationTransport)

	req, err := http.NewRequest("GET", "http://example.com", nil)
	assert.Nil(err)
	assert.Empty(req.Header.Get("Authorization"), "authorization header key must empty")

	resp, err := authorizationTransport.RoundTrip(req)
	assert.Nil(err)
	assert.NotEmpty(req.Header.Get("Authorization"), "authorization header key must not empty")
	assert.NotNil(resp)

	auth := "um6IEH7mtwnwkGpjImD08JdxlvViuELhI4m"
	assert.Equal(req.Header.Get("Authorization"), auth, "Two authorization header key should be the same.")
}
