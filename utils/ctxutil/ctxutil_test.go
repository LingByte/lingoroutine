package ctxutil

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"context"
	"testing"
	"time"
)

func TestGetRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = SetRequestID(ctx, "req-123")

	reqID := GetRequestID(ctx)
	if reqID != "req-123" {
		t.Errorf("expected 'req-123', got '%s'", reqID)
	}
}

func TestGetTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = SetTraceID(ctx, "trace-456")

	traceID := GetTraceID(ctx)
	if traceID != "trace-456" {
		t.Errorf("expected 'trace-456', got '%s'", traceID)
	}
}

func TestGetClientIP(t *testing.T) {
	ctx := context.Background()
	ctx = SetClientIP(ctx, "192.168.1.1")

	ip := GetClientIP(ctx)
	if ip != "192.168.1.1" {
		t.Errorf("expected '192.168.1.1', got '%s'", ip)
	}
}

func TestGetUserAgent(t *testing.T) {
	ctx := context.Background()
	ctx = SetUserAgent(ctx, "Mozilla/5.0")

	ua := GetUserAgent(ctx)
	if ua != "Mozilla/5.0" {
		t.Errorf("expected 'Mozilla/5.0', got '%s'", ua)
	}
}

func TestWithTimeout(t *testing.T) {
	ctx := context.Background()

	// 没有 deadline 的情况
	timeoutCtx, cancel := WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	select {
	case <-timeoutCtx.Done():
		if timeoutCtx.Err() != context.DeadlineExceeded {
			t.Error("expected deadline exceeded")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("should timeout")
	}

	// 已有 deadline 的情况
	deadlineCtx, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	// 不会覆盖已有的 deadline
	newCtx, cancel3 := WithTimeout(deadlineCtx, 200*time.Millisecond)
	defer cancel3()

	select {
	case <-newCtx.Done():
		// 应该在 50ms 超时，而不是 200ms
	case <-time.After(100 * time.Millisecond):
		t.Error("should timeout at 50ms")
	}
}

func TestRequestContext(t *testing.T) {
	ctx := context.Background()
	ctx = SetRequestID(ctx, "req-789")
	ctx = SetTraceID(ctx, "trace-xyz")
	ctx = SetClientIP(ctx, "127.0.0.1")
	ctx = SetUserAgent(ctx, "TestAgent")

	reqCtx := BuildRequestContext(ctx)

	if reqCtx.RequestID != "req-789" {
		t.Errorf("expected 'req-789', got '%s'", reqCtx.RequestID)
	}
	if reqCtx.TraceID != "trace-xyz" {
		t.Errorf("expected 'trace-xyz', got '%s'", reqCtx.TraceID)
	}
	if reqCtx.ClientIP != "127.0.0.1" {
		t.Errorf("expected '127.0.0.1', got '%s'", reqCtx.ClientIP)
	}
	if reqCtx.UserAgent != "TestAgent" {
		t.Errorf("expected 'TestAgent', got '%s'", reqCtx.UserAgent)
	}
	if !reqCtx.IsInternal {
		t.Error("127.0.0.1 should be internal IP")
	}
	if reqCtx.Country != "Local" {
		t.Errorf("expected 'Local', got '%s'", reqCtx.Country)
	}
}

func TestSetGetRequestContext(t *testing.T) {
	ctx := context.Background()

	originalReqCtx := &RequestContext{
		RequestID:   "req-abc",
		TraceID:     "trace-def",
		ClientIP:    "192.168.1.1",
		UserAgent:   "Test",
		Country:     "Local",
		City:        "Local",
		Location:    "Local Network",
		IsInternal:  true,
		RequestTime: time.Now(),
	}

	ctx = SetRequestContext(ctx, originalReqCtx)

	retrieved := GetRequestContext(ctx)
	if retrieved == nil {
		t.Fatal("retrieved context should not be nil")
	}
	if retrieved.RequestID != originalReqCtx.RequestID {
		t.Errorf("expected '%s', got '%s'", originalReqCtx.RequestID, retrieved.RequestID)
	}
	if retrieved.ClientIP != originalReqCtx.ClientIP {
		t.Errorf("expected '%s', got '%s'", originalReqCtx.ClientIP, retrieved.ClientIP)
	}
}
