package ctxutil

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"context"
	"time"

	"github.com/LingByte/lingoroutine/utils"
	"github.com/gin-gonic/gin"
)

// ContextKey 自定义context key类型，避免key冲突
type ContextKey string

// RequestContext 请求上下文结构体，封装请求相关的所有信息
type RequestContext struct {
	RequestID   string    // 请求ID
	TraceID     string    // 链路追踪ID
	ClientIP    string    // 客户端IP
	UserAgent   string    // 用户代理
	Country     string    // 国家
	City        string    // 城市
	Location    string    // 完整位置
	IsInternal  bool      // 是否为内网IP
	RequestTime time.Time // 请求时间
}

const (
	// KeyRequestID 请求ID
	KeyRequestID ContextKey = "request_id"
	// KeyTraceID 链路追踪ID
	KeyTraceID ContextKey = "trace_id"
	// KeyClientIP 客户端IP
	KeyClientIP ContextKey = "client_ip"
	// KeyUserAgent 用户代理
	KeyUserAgent ContextKey = "user_agent"
	// KeyRequestContext 请求上下文结构体
	KeyRequestContext ContextKey = "request_context"
)

// GetRequestID 获取请求ID
func GetRequestID(ctx context.Context) string {
	if val, ok := ctx.Value(KeyRequestID).(string); ok {
		return val
	}
	return ""
}

// SetRequestID 设置请求ID
func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, KeyRequestID, requestID)
}

// GetTraceID 获取链路追踪ID
func GetTraceID(ctx context.Context) string {
	if val, ok := ctx.Value(KeyTraceID).(string); ok {
		return val
	}
	return ""
}

// SetTraceID 设置链路追踪ID
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, KeyTraceID, traceID)
}

// GetClientIP 获取客户端IP
func GetClientIP(ctx context.Context) string {
	if val, ok := ctx.Value(KeyClientIP).(string); ok {
		return val
	}
	return ""
}

// SetClientIP 设置客户端IP
func SetClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, KeyClientIP, ip)
}

// WithTimeout 包装 context.WithTimeout，添加默认超时
func WithTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		// 如果已经有 deadline，不覆盖
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

// BuildRequestContextFromGin 从 gin.Context 构建请求上下文
func BuildRequestContextFromGin(c *gin.Context) *RequestContext {
	ctx := &RequestContext{
		RequestTime: time.Now(),
	}
	// 获取客户端IP
	ctx.ClientIP = c.ClientIP()
	ctx.IsInternal = utils.IsInternalIP(ctx.ClientIP)
	// 获取 User-Agent
	ctx.UserAgent = c.GetHeader("User-Agent")
	// 获取地理位置（仅对外网IP）
	if !ctx.IsInternal {
		country, city, location := utils.GetLocationByIP(ctx.ClientIP)
		ctx.Country = country
		ctx.City = city
		ctx.Location = location
	} else {
		ctx.Country = "Local"
		ctx.City = "Local"
		ctx.Location = "Local Network"
	}
	return ctx
}

// BuildRequestContext 从标准库 context 构建请求上下文
func BuildRequestContext(ctx context.Context) *RequestContext {
	reqCtx := &RequestContext{
		RequestTime: time.Now(),
	}
	// 从 context 中获取已设置的值
	reqCtx.RequestID = GetRequestID(ctx)
	reqCtx.TraceID = GetTraceID(ctx)
	reqCtx.ClientIP = GetClientIP(ctx)
	reqCtx.UserAgent = GetUserAgent(ctx)
	// 获取地理位置
	if reqCtx.ClientIP != "" {
		reqCtx.IsInternal = utils.IsInternalIP(reqCtx.ClientIP)
		if !reqCtx.IsInternal {
			country, city, location := utils.GetLocationByIP(reqCtx.ClientIP)
			reqCtx.Country = country
			reqCtx.City = city
			reqCtx.Location = location
		} else {
			reqCtx.Country = "Local"
			reqCtx.City = "Local"
			reqCtx.Location = "Local Network"
		}
	}
	return reqCtx
}

// GetUserAgent 获取用户代理
func GetUserAgent(ctx context.Context) string {
	if val, ok := ctx.Value(KeyUserAgent).(string); ok {
		return val
	}
	return ""
}

// SetUserAgent 设置用户代理
func SetUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, KeyUserAgent, userAgent)
}

// GetRequestContext 从 context 获取请求上下文结构体
func GetRequestContext(ctx context.Context) *RequestContext {
	if val, ok := ctx.Value(KeyRequestContext).(*RequestContext); ok {
		return val
	}
	return nil
}

// SetRequestContext 设置请求上下文结构体到 context
func SetRequestContext(ctx context.Context, reqCtx *RequestContext) context.Context {
	return context.WithValue(ctx, KeyRequestContext, reqCtx)
}
