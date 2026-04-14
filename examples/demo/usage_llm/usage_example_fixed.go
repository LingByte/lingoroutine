package main

import (
	"context"
	"log"
	"time"

	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/models"
	"github.com/LingByte/lingoroutine/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 初始化数据库
	db, err := gorm.Open(sqlite.Open("chat.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// 自动迁移数据库表
	err = db.AutoMigrate(
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.LLMUsage{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// 连接信号处理（纯函数方式）
	utils.Sig().Connect("LLMUsage", func(sender any, params ...any) {
		if len(params) >= 3 {
			usageInfo := params[0].(map[string]interface{})
			_ = params[1].(string) // text
			_ = params[2].(string) // finalResponse

			// 保存到数据库
			usage := &models.LLMUsage{
				ID:           utils.SnowflakeUtil.GenID(),
				RequestID:    usageInfo["request_id"].(string),
				SessionID:    usageInfo["session_id"].(string),
				UserID:       usageInfo["user_id"].(string),
				Provider:     usageInfo["provider"].(string),
				Model:        usageInfo["model"].(string),
				RequestType:  usageInfo["request_type"].(string),
				InputTokens:  int(usageInfo["input_tokens"].(int64)),
				OutputTokens: int(usageInfo["output_tokens"].(int64)),
				TotalTokens:  int(usageInfo["total_tokens"].(int64)),
				LatencyMs:    usageInfo["latency_ms"].(int64),
				Success:      usageInfo["success"].(bool),
				RequestedAt:  time.Unix(usageInfo["requested_at"].(int64)/1000, 0),
				CompletedAt:  time.Unix(usageInfo["completed_at"].(int64)/1000, 0),
			}

			if err := db.Create(usage).Error; err != nil {
				log.Printf("Failed to save LLM usage: %v", err)
			} else {
				log.Printf("已保存LLM用量记录 - ID: %s", usage.ID)
			}

			log.Printf("收到LLM用量信号: %s", usageInfo["request_id"])
			log.Printf("   - 输入Token: %d", usageInfo["input_tokens"])
			log.Printf("   - 输出Token: %d", usageInfo["output_tokens"])
			log.Printf("   - 延迟: %d ms", usageInfo["latency_ms"])
		}
	})

	utils.Sig().Connect("SessionCreated", func(sender any, params ...any) {
		if len(params) > 0 {
			sessionInfo := params[0].(map[string]interface{})

			session := &models.ChatSession{
				ID:           utils.SnowflakeUtil.GenID(),
				UserID:       sessionInfo["user_id"].(string),
				Title:        sessionInfo["title"].(string),
				Provider:     sessionInfo["provider"].(string),
				Model:        sessionInfo["model"].(string),
				SystemPrompt: sessionInfo["system_prompt"].(string),
				Status:       "active",
				CreatedAt:    time.Unix(sessionInfo["created_at"].(int64)/1000, 0),
			}

			if err := db.Create(session).Error; err != nil {
				log.Printf("Failed to save chat session: %v", err)
			} else {
				log.Printf("已保存聊天会话 - ID: %s", session.ID)
			}
		}
	})

	utils.Sig().Connect("MessageCreated", func(sender any, params ...any) {
		if len(params) > 0 {
			messageInfo := params[0].(map[string]interface{})

			message := &models.ChatMessage{
				ID:         utils.SnowflakeUtil.GenID(),
				SessionID:  messageInfo["session_id"].(string),
				Role:       messageInfo["role"].(string),
				Content:    messageInfo["content"].(string),
				TokenCount: int(messageInfo["token_count"].(int64)),
				Model:      messageInfo["model"].(string),
				Provider:   messageInfo["provider"].(string),
				RequestID:  messageInfo["request_id"].(string),
				CreatedAt:  time.Unix(messageInfo["created_at"].(int64)/1000, 0),
			}

			if err := db.Create(message).Error; err != nil {
				log.Printf("Failed to save chat message: %v", err)
			}
		}
	})

	// 创建LLM处理器
	provider := utils.GetEnv("LLM_PROVIDER")
	if provider == "" {
		provider = "lmstudio"
	}

	baseURL := utils.GetEnv("LLM_BASEURL")
	if baseURL == "" {
		baseURL = "http://localhost:1234/v1"
	}

	apiKey := utils.GetEnv("LLM_API_KEY")
	if apiKey == "" {
		apiKey = "lmstudio"
	}

	handler, err := llm.NewLLMProvider(context.Background(),
		provider,
		apiKey,
		baseURL,
		"",
	)
	if err != nil {
		log.Fatal("Failed to create LLM handler:", err)
	}

	// 生成会话ID和用户ID
	sessionID := utils.SnowflakeUtil.GenID()
	userID := "user123"

	// 获取模型名称
	model := utils.GetEnv("LLM_MODEL")
	if model == "" {
		model = "local-model"
	}

	// 创建会话
	llm.CreateSession(
		sessionID,
		userID,
		"测试会话",
		handler.Provider(),
		model,
		"你是一个有用的助手",
	)

	// 发送查询请求
	response, err := handler.QueryWithOptions("你好，请介绍一下你自己", &llm.QueryOptions{
		Model:       model,
		SessionID:   sessionID,
		UserID:      userID,
		RequestType: "query",
	})
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}

	// 创建用户消息
	userMessageID := utils.SnowflakeUtil.GenID()
	llm.CreateMessage(
		userMessageID,
		sessionID,
		"user",
		"你好，请介绍一下你自己",
		0, // token count，可以后续计算
		model,
		handler.Provider(),
		response.RequestID,
	)

	// 创建助手消息
	assistantMessageID := utils.SnowflakeUtil.GenID()
	if len(response.Choices) > 0 {
		llm.CreateMessage(
			assistantMessageID,
			sessionID,
			"assistant",
			response.Choices[0].Content,
			response.Usage.CompletionTokens,
			model,
			handler.Provider(),
			response.RequestID,
		)
	}

	log.Printf("Response: %s", response.Choices[0].Content)
	log.Printf("Request ID: %s", response.RequestID)
	log.Printf("Latency: %d ms", response.LatencyMs)
	log.Printf("Tokens used: %d", response.Usage.TotalTokens)
}
