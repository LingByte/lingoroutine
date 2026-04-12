package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"

	"github.com/LingByte/lingoroutine/utils/ctxutil"
	"github.com/gin-gonic/gin"
)

func main() {
	fmt.Println("=== ctxutil 使用示例 ===")
	fmt.Println()

	// 1. 从标准库 context 构建请求上下文
	fmt.Println("1. 从标准库 context 构建请求上下文:")
	ctx := context.Background()
	ctx = ctxutil.SetRequestID(ctx, "req-123")
	ctx = ctxutil.SetTraceID(ctx, "trace-456")
	ctx = ctxutil.SetClientIP(ctx, "8.8.8.8")
	ctx = ctxutil.SetUserAgent(ctx, "Mozilla/5.0")

	reqCtx := ctxutil.BuildRequestContext(ctx)
	fmt.Printf("   RequestID: %s\n", reqCtx.RequestID)
	fmt.Printf("   TraceID: %s\n", reqCtx.TraceID)
	fmt.Printf("   ClientIP: %s\n", reqCtx.ClientIP)
	fmt.Printf("   UserAgent: %s\n", reqCtx.UserAgent)
	fmt.Printf("   Country: %s\n", reqCtx.Country)
	fmt.Printf("   City: %s\n", reqCtx.City)
	fmt.Printf("   Location: %s\n", reqCtx.Location)
	fmt.Printf("   IsInternal: %v\n", reqCtx.IsInternal)
	fmt.Println()

	// 2. 从 gin.Context 构建请求上下文
	fmt.Println("2. 从 gin.Context 构建请求上下文:")
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		reqCtx := ctxutil.BuildRequestContextFromGin(c)
		c.JSON(200, gin.H{
			"request_id":   reqCtx.RequestID,
			"client_ip":    reqCtx.ClientIP,
			"user_agent":   reqCtx.UserAgent,
			"country":      reqCtx.Country,
			"city":         reqCtx.City,
			"location":     reqCtx.Location,
			"is_internal":  reqCtx.IsInternal,
			"request_time": reqCtx.RequestTime,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	req.RemoteAddr = "1.1.1.1:1234"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	fmt.Printf("   Response: %s\n", w.Body.String())
	fmt.Println()

	// 3. 将 RequestContext 存储到 context 中
	fmt.Println("3. 将 RequestContext 存储到 context 中:")
	ctx2 := context.Background()
	reqCtx2 := &ctxutil.RequestContext{
		RequestID:   "req-789",
		TraceID:     "trace-xyz",
		ClientIP:    "192.168.1.1",
		UserAgent:   "CustomAgent",
		Country:     "Local",
		City:        "Local",
		Location:    "Local Network",
		IsInternal:  true,
		RequestTime: time.Now(),
	}

	ctx2 = ctxutil.SetRequestContext(ctx2, reqCtx2)
	retrieved := ctxutil.GetRequestContext(ctx2)

	if retrieved != nil {
		fmt.Printf("   Retrieved RequestID: %s\n", retrieved.RequestID)
		fmt.Printf("   Retrieved ClientIP: %s\n", retrieved.ClientIP)
	}
	fmt.Println()

	// 4. WithTimeout 使用
	fmt.Println("4. WithTimeout 使用:")
	timeoutCtx, cancel := ctxutil.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	select {
	case <-timeoutCtx.Done():
		fmt.Printf("   Context cancelled: %v\n", timeoutCtx.Err())
	case <-time.After(1 * time.Second):
		fmt.Println("   Operation completed before timeout")
	}
}
