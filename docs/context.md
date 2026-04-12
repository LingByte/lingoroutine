# Context 使用指南

## 封装说明

本项目使用 `utils/ctxutil` 包来封装 Context 相关的工具函数，避免与标准库的 `context` 包冲突。

## Context 类型对比

### 1. 标准库 context.Context

Go 标准库的 `context.Context` 是一个接口，用于在 API 边界和进程间传递请求范围的数据、取消信号、截止时间。

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key interface{}) interface{}
}
```

### 2. context.Background()

**用途**: 作为根 context 使用

**场景**:
- 主程序初始化
- 测试用例
- 顶层请求处理
- 无法从父 context 派生的场景

**特点**:
- 永远不会被取消
- 没有值
- 没有截止时间

```go
ctx := context.Background()
```

### 3. context.TODO()

**用途**: 当不确定使用什么 context 时使用

**场景**:
- 代码还在开发中
- 暂时不确定 context 来源
- 后续会替换为具体的 context

**特点**:
- 与 Background 类似，但语义不同
- 表示"待确定"

```go
func someFunction() {
    ctx := context.TODO() // 后续会替换为传入的参数
    // ...
}
```

### 4. gin.Context

**用途**: Gin 框架的 HTTP 请求上下文

**场景**:
- HTTP 请求处理
- Web API 开发
- 需要 HTTP 特定功能的场景

**特点**:
- 包含 HTTP 请求和响应信息
- 可以获取请求参数、头信息等
- 内置标准库 context，可通过 `c.Request.Context()` 获取

```go
func handler(c *gin.Context) {
    // gin.Context 包含 HTTP 特定功能
    userID := c.Param("id")
    c.JSON(200, gin.H{"user_id": userID})

    // 获取标准库 context
    stdCtx := c.Request.Context()
}
```

## Context 传递规则

### 1. 不要在结构体中存储 Context

❌ 错误:
```go
type MyStruct struct {
    ctx context.Context // 不要这样做
}
```

✅ 正确:
```go
func (s *MyStruct) DoSomething(ctx context.Context) error {
    // 作为参数传递
}
```

### 2. Context 作为第一个参数

```go
func DoSomething(ctx context.Context, arg1, arg2 string) error {
    // ctx 作为第一个参数
}
```

### 3. 不要传递 nil Context

如果不确定使用什么 context，使用 `context.TODO()`

## 实际使用场景

### 场景 1: HTTP 请求处理

```go
import "github.com/LingByte/lingoroutine/utils/ctxutil"

func handleRequest(c *gin.Context) {
    // 从 gin.Context 获取标准库 context
    ctx := c.Request.Context()

    // 添加请求追踪信息
    ctx = ctxutil.SetRequestID(ctx, generateRequestID())
    ctx = ctxutil.SetUserID(ctx, getUserIDFromToken(c))

    // 传递给业务逻辑
    result, err := businessLogic(ctx)
}
```

### 场景 2: 数据库查询

```go
import "github.com/LingByte/lingoroutine/utils/ctxutil"

func queryUser(ctx context.Context, userID uint) (*User, error) {
    // 设置超时（如果没有 deadline 才设置）
    ctx, cancel := ctxutil.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // 传递给数据库查询
    return db.WithContext(ctx).Where("id = ?", userID).First(&user)
}
```

### 场景 3: 外部 API 调用

```go
import "github.com/LingByte/lingoroutine/utils/ctxutil"

func callExternalAPI(ctx context.Context, req *Request) (*Response, error) {
    // 设置截止时间
    deadline := time.Now().Add(10 * time.Second)
    ctx, cancel := context.WithDeadline(ctx, deadline)
    defer cancel()

    // HTTP 请求
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    // ...
}
```

### 场景 4: 后台任务

```go
func startBackgroundTask() {
    // 使用 Background context
    ctx := context.Background()

    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-time.Tick(1 * time.Minute):
                processTask(ctx)
            }
        }
    }()
}
```

## 封装建议

### 1. 统一的 Context Key

使用自定义类型避免 key 冲突（已在 ctxutil 包中定义）:

```go
import "github.com/LingByte/lingoroutine/utils/ctxutil"

const (
    KeyUserID ctxutil.ContextKey = "user_id"
    KeyRequestID ctxutil.ContextKey = "request_id"
)
```

### 2. 类型安全的 getter/setter

```go
import "github.com/LingByte/lingoroutine/utils/ctxutil"

userID := ctxutil.GetUserID(ctx)
ctx = ctxutil.SetUserID(ctx, 123)
```

### 3. 链式调用

```go
import "github.com/LingByte/lingoroutine/utils/ctxutil"

ctx = ctxutil.SetUserID(ctx, 123)
ctx = ctxutil.SetUsername(ctx, "test")
ctx = ctxutil.SetRequestID(ctx, "req-123")
```

## 最佳实践

1. **总是传递 context**: 即使当前不使用，也要传递给下游
2. **设置合理的超时**: 避免资源长时间占用
3. **及时调用 cancel**: 使用 defer 确保资源释放
4. **不要在 context 中存储大量数据**: context 应该只存储请求范围的元数据
5. **使用类型安全的 key**: 避免字符串 key 冲突
