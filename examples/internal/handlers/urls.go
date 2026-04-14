package handlers

import (
	"github.com/LingByte/CinyuVerse/pkg/config"
	"github.com/LingByte/lingoroutine/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

type CinyuHandlers struct {
	db *gorm.DB
}

func NewCinyuHandlers(db *gorm.DB) *CinyuHandlers {
	return &CinyuHandlers{
		db: db,
	}
}

func (ch *CinyuHandlers) RegisterHandlers(engine *gin.Engine) {
	// 安全检查 API 前缀
	apiPrefix := "/api"
	if config.GlobalConfig != nil && config.GlobalConfig.Server.APIPrefix != "" {
		apiPrefix = config.GlobalConfig.Server.APIPrefix
	}
	
	r := engine.Group(apiPrefix)

	// Register Global Singleton DB
	r.Use(middleware.InjectDB(ch.db))

	// 信号连接已在main.go中初始化，这里不再重复初始化
	
	// LLM 聊天接口
	r.POST("/llm/chat", ch.LLMChat)
	r.POST("/llm/chat/stream", ch.LLMChatStream)
	
	// 数据查询接口
	dataAPI := NewLLMDataAPI(ch.db)
	r.GET("/llm/sessions", dataAPI.GetSessions)
	r.POST("/llm/sessions", dataAPI.CreateSession)
	r.GET("/llm/sessions/:sessionId/messages", dataAPI.GetSessionMessages)
	r.GET("/llm/messages", dataAPI.GetMessages)
	r.GET("/llm/usage", dataAPI.GetUsage)
}
