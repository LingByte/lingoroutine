package handlers

import (
	"strconv"

	"github.com/LingByte/lingoroutine/models"
	"github.com/LingByte/lingoroutine/response"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LLMDataAPI 数据查询接口
type LLMDataAPI struct {
	db *gorm.DB
}

func NewLLMDataAPI(db *gorm.DB) *LLMDataAPI {
	return &LLMDataAPI{db: db}
}

// GetSessions 获取会话列表
func (api *LLMDataAPI) GetSessions(c *gin.Context) {
	var sessions []models.ChatSession
	if err := api.db.Order("created_at DESC").Find(&sessions).Error; err != nil {
		response.Fail(c, "查询会话失败", gin.H{"error": err.Error()})
		return
	}
	response.Success(c, "ok", sessions)
}

// GetMessages 获取消息列表
func (api *LLMDataAPI) GetMessages(c *gin.Context) {
	var messages []models.ChatMessage
	if err := api.db.Order("created_at DESC").Find(&messages).Error; err != nil {
		response.Fail(c, "查询消息失败", gin.H{"error": err.Error()})
		return
	}
	response.Success(c, "ok", messages)
}

// GetUsage 获取用量统计
func (api *LLMDataAPI) GetUsage(c *gin.Context) {
	var usage []models.LLMUsage
	query := api.db.Order("completed_at DESC")
	
	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if size > 200 {
		size = 200
	}
	offset := (page - 1) * size
	
	if err := query.Offset(offset).Limit(size).Find(&usage).Error; err != nil {
		response.Fail(c, "查询用量失败", gin.H{"error": err.Error()})
		return
	}
	
	// 获取总数
	var total int64
	api.db.Model(&models.LLMUsage{}).Count(&total)
	
	response.Success(c, "ok", gin.H{
		"list":  usage,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetSessionMessages 获取指定会话的消息
func (api *LLMDataAPI) GetSessionMessages(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		response.FailWithCode(c, 400, "session_id is required", nil)
		return
	}
	
	var messages []models.ChatMessage
	if err := api.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&messages).Error; err != nil {
		response.Fail(c, "查询会话消息失败", gin.H{"error": err.Error()})
		return
	}
	response.Success(c, "ok", messages)
}

// CreateSession 创建会话
func (api *LLMDataAPI) CreateSession(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
		Title  string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, 400, "invalid request", gin.H{"error": err.Error()})
		return
	}
	
	session := &models.ChatSession{
		ID:       utils.SnowflakeUtil.GenID(),
		UserID:   req.UserID,
		Title:    req.Title,
		Status:   "active",
	}
	
	if err := api.db.Create(session).Error; err != nil {
		response.Fail(c, "创建会话失败", gin.H{"error": err.Error()})
		return
	}
	
	response.Success(c, "ok", session)
}
