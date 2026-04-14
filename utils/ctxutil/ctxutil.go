package ctxutil

import (
	"context"
	"time"

	"github.com/LingByte/lingoroutine/utils"
	"github.com/gin-gonic/gin"
)

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

// Golang中所谓的上下文Context是在一组协程中传递信号量，用于进行超时,取消和少量数据的传递
// 官方是如此介绍的，Context 用于在 API 边界 以及进程之间传递截止时间、取消信号和其他请求级别的值。
// 进入服务器的请求应当创建一个 [Context]，而对服务器的外发调用
// 应当接收一个 Context。它们之间的函数调用链必须传播该 Context，
// 也可以选择使用 [WithCancel]、[WithDeadline]、[WithTimeout]
// 或 [WithValue] 创建的派生 Context 替换它。
//
// 取消一个 Context 表示代表其执行的工作应当停止。
// 带有截止时间的 Context 会在截止时间到达后被取消。
// 当一个 Context 被取消时，从它派生的所有 Context 也会被取消。

// [WithCancel]、[WithDeadline] 和 [WithTimeout] 函数接收一个
// Context（父上下文）并返回一个派生 Context（子上下文）
// 以及一个 [CancelFunc]。直接调用 CancelFunc 会取消该子上下文
// 及其所有后代，移除父上下文对子上下文的引用，并停止
// 所有关联的定时器。如果不调用 CancelFunc，会导致子上下文
// 及其后代泄漏，直到父上下文被取消。go vet 工具会检查
// CancelFunc 是否在所有控制流路径上都被使用。
//
// [WithCancelCause]、[WithDeadlineCause] 和 [WithTimeoutCause]
// 函数返回一个 [CancelCauseFunc]，它接收一个 error 并将其记录
// 为取消原因。在已取消的上下文或其任意后代上调用 [Cause]
// 可以获取该原因。如果未指定原因，Cause(ctx) 返回的值与 ctx.Err() 相同。
//
// 使用 Context 的程序应遵循以下规则，以保持各包之间接口的一致性，
// 并使静态分析工具能够检查上下文传播：
//
// 不要将 Context 存储在结构体类型内部；相反，将 Context
// 显式传递给每个需要它的函数。更多讨论见
// https://go.dev/blog/context-and-structs。Context 应作为第一个参数，
// 通常命名为 ctx：
//
//	func DoSomething(ctx context.Context, arg Arg) error {
//		// ... 使用 ctx ...
//	}
//
// 即使函数允许，也不要传递 nil [Context]。如果不确定使用哪个 Context，
// 请传递 [context.TODO]。
//
// 仅将 context Values 用于跨进程和 API 传递的请求级数据，
// 不要用于向函数传递可选参数。
//
// 同一个 Context 可以传递给运行在不同 goroutine 中的函数；
// Context 可安全地被多个 goroutine 同时使用。
//
// 有关使用 Context 的服务器示例代码，参见 https://go.dev/blog/context。

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
