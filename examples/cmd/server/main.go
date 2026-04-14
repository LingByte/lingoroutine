package main

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LingByte/lingoroutine/bootstrap"
	"github.com/LingByte/lingoroutine/examples"
	"github.com/LingByte/lingoroutine/examples/internal/handlers"
	"github.com/LingByte/lingoroutine/examples/pkg/config"
	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/logger"
	"github.com/LingByte/lingoroutine/middleware"
	"github.com/LingByte/lingoroutine/models"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CinyuVerseApp struct {
	db       *gorm.DB
	handlers *handlers.CinyuHandlers
}

func NewCinyuVerseApp(db *gorm.DB) *CinyuVerseApp {
	return &CinyuVerseApp{
		db:       db,
		handlers: handlers.NewCinyuHandlers(db),
	}
}

func (app *CinyuVerseApp) RegisterRoutes(r *gin.Engine) {
	// Register system routes (with /api prefix)
	app.handlers.RegisterHandlers(r)
}

// 初始化信号连接
func initSignalConnections(db *gorm.DB) {
	// LLMUsage 信号连接
	utils.Sig().Connect("LLMUsage", func(sender any, params ...any) {
		usageInfo, ok := sender.(map[string]interface{})
		if !ok {
			return
		}

		requestID := asString(usageInfo["request_id"])
		if requestID == "" {
			return
		}

		// 检查是否已存在相同 request_id 的记录
		var existingUsage models.LLMUsage
		if err := db.Where("request_id = ?", requestID).First(&existingUsage).Error; err == nil {
			// 记录已存在，跳过插入
			return
		}

		usage := &models.LLMUsage{
			ID:          utils.SnowflakeUtil.GenID(),
			RequestID:   requestID,
			SessionID:   asString(usageInfo["session_id"]),
			UserID:      asString(usageInfo["user_id"]),
			Provider:    asString(usageInfo["provider"]),
			Model:       asString(usageInfo["model"]),
			BaseURL:     asString(usageInfo["base_url"]),
			RequestType: asString(usageInfo["request_type"]),

			// Token统计
			InputTokens:  asInt(usageInfo["input_tokens"]),
			OutputTokens: asInt(usageInfo["output_tokens"]),
			TotalTokens:  asInt(usageInfo["total_tokens"]),

			// 性能指标
			LatencyMs:   asInt64(usageInfo["latency_ms"]),
			TTFTMs:      asInt64(usageInfo["ttft_ms"]),
			TPS:         asFloat64(usageInfo["tps"]),
			QueueTimeMs: asInt64(usageInfo["queue_time_ms"]),

			// 请求响应内容
			RequestContent:  asString(usageInfo["request_content"]),
			ResponseContent: asString(usageInfo["response_content"]),

			// 请求元信息
			UserAgent:  asString(usageInfo["user_agent"]),
			IPAddress:  asString(usageInfo["ip_address"]),
			StatusCode: asInt(usageInfo["status_code"]),

			// 错误信息
			Success:      asBool(usageInfo["success"]),
			ErrorCode:    asString(usageInfo["error_code"]),
			ErrorMessage: asString(usageInfo["error_message"]),

			// 时间戳
			RequestedAt:  time.Unix(asInt64(usageInfo["requested_at"])/1000, 0),
			StartedAt:    time.Unix(asInt64(usageInfo["started_at"])/1000, 0),
			FirstTokenAt: time.Unix(asInt64(usageInfo["first_token_at"])/1000, 0),
			CompletedAt:  time.Unix(asInt64(usageInfo["completed_at"])/1000, 0),
		}
		_ = db.Create(usage).Error
	})

	// SessionCreated 信号连接
	utils.Sig().Connect(llm.SignalSessionCreated, func(sender any, params ...any) {
		if len(params) < 1 {
			return
		}
		data, ok := params[0].(llm.SessionCreatedData)
		if !ok {
			return
		}
		session := &models.ChatSession{
			ID:           utils.SnowflakeUtil.GenID(),
			UserID:       data.UserID,
			Title:        data.Title,
			Provider:     data.Provider,
			Model:        data.Model,
			SystemPrompt: data.SystemPrompt,
			Status:       "active",
			CreatedAt:    time.Unix(data.CreatedAt/1000, 0),
		}
		_ = db.Create(session).Error
	})

	// MessageCreated 信号连接
	utils.Sig().Connect(llm.SignalMessageCreated, func(sender any, params ...any) {
		if len(params) < 1 {
			return
		}
		data, ok := params[0].(llm.MessageCreatedData)
		if !ok {
			return
		}
		message := &models.ChatMessage{
			ID:         utils.SnowflakeUtil.GenID(),
			SessionID:  data.SessionID,
			Role:       data.Role,
			Content:    data.Content,
			TokenCount: data.TokenCount,
			Model:      data.Model,
			Provider:   data.Provider,
			RequestID:  data.RequestID,
			CreatedAt:  time.Unix(data.CreatedAt/1000, 0),
		}
		_ = db.Create(message).Error
	})
}

// 辅助函数
func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

var asFloat64 = func(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func asInt(v interface{}) int {
	if i, ok := v.(int); ok {
		return i
	}
	return 0
}

func asInt64(v interface{}) int64 {
	if i, ok := v.(int64); ok {
		return i
	}
	return 0
}

func asBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func main() {
	if err := config.Load(); err != nil {
		log.Fatalf("config load: %v", err)
	}
	if err := config.GlobalConfig.Validate(); err != nil {
		log.Fatalf("config validate: %v", err)
	}
	logDir := filepath.Dir(config.GlobalConfig.Log.Filename)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Fatalf("mkdir logs: %v", err)
	}
	if !strings.Contains(config.GlobalConfig.Database.DSN, ":memory:") {
		if d := filepath.Dir(config.GlobalConfig.Database.DSN); d != "." && d != "" {
			if err := os.MkdirAll(d, 0o755); err != nil {
				log.Fatalf("mkdir database dir: %v", err)
			}
		}
	}
	if err := logger.Init(&config.GlobalConfig.Log, config.GlobalConfig.LogMode()); err != nil {
		log.Fatalf("init logger: %v", err)
	}
	bs := bootstrap.NewBootstrap(os.Stdout, &bootstrap.Options{
		DBDriver:      config.GlobalConfig.Database.Driver,
		DSN:           config.GlobalConfig.Database.DSN,
		AutoMigrate:   true,
		SeedNonProd:   false,
		MigrationsDir: "migrations",
		Models: []any{
			&models.ChatSession{},
			&models.ChatMessage{},
			&models.LLMUsage{},
			&models.AgentRun{},
			&models.AgentStep{},
		},
	})
	db, err := bs.SetupDatabase()
	if err != nil {
		logger.Lg.Fatal("database bootstrap", zap.Error(err))
	}

	if config.GlobalConfig.Server.Mode == "production" || config.GlobalConfig.Server.Mode == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.InjectDB(db))
	r.Use(middleware.CorsMiddleware())
	initSignalConnections(db)
	// 静态文件服务 - 使用embed
	staticFS, err := fs.Sub(examples.StaticFiles, "static")
	if err != nil {
		logger.Lg.Fatal("failed to create static filesystem", zap.Error(err))
	}

	// 设置HTML模板
	tmpl := template.Must(template.New("").ParseFS(examples.StaticFiles, "static/*.html"))
	r.SetHTMLTemplate(tmpl)

	// 静态文件路由
	r.StaticFS("/static", http.FS(staticFS))
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	app := NewCinyuVerseApp(db)
	app.RegisterRoutes(r)
	addr := config.GlobalConfig.Server.Addr
	logger.Lg.Info("http listening", zap.String("addr", addr))
	logger.Lg.Info("web UI available at", zap.String("url", "http://"+addr))
	logger.Lg.Info("chat page at", zap.String("url", "http://"+addr+"/static/chat.html"))
	logger.Lg.Info("data dashboard at", zap.String("url", "http://"+addr+"/static/data.html"))
	if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
		logger.Lg.Fatal("http server", zap.Error(err))
	}
}
