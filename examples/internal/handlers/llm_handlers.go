package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LingByte/lingoroutine/examples/pkg/config"
	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/models"
	"github.com/LingByte/lingoroutine/response"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type llmChatRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Input     string `json:"input"`
	Model     string `json:"model"`
}

type agentChatRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Input     string `json:"input"`
	Model     string `json:"model"`
	MaxTasks  int    `json:"max_tasks"`
}

type llmChatResponse struct {
	SessionID string          `json:"session_id"`
	RequestID string          `json:"request_id"`
	LatencyMs int64           `json:"latency_ms"`
	Output    string          `json:"output"`
	Usage     *llm.TokenUsage `json:"usage"`
}

var initSigOnce sync.Once

func initSignalPersistence(db *gorm.DB) {
	initSigOnce.Do(func() {
		utils.Sig().Connect("LLMUsage", func(sender any, params ...any) {
			usageInfo, ok := sender.(map[string]interface{})
			if !ok {
				return
			}
			usage := &models.LLMUsage{
				ID:           utils.SnowflakeUtil.GenID(),
				RequestID:    asString(usageInfo["request_id"]),
				SessionID:    asString(usageInfo["session_id"]),
				UserID:       asString(usageInfo["user_id"]),
				Provider:     asString(usageInfo["provider"]),
				Model:        asString(usageInfo["model"]),
				RequestType:  asString(usageInfo["request_type"]),
				InputTokens:  asInt(usageInfo["input_tokens"]),
				OutputTokens: asInt(usageInfo["output_tokens"]),
				TotalTokens:  asInt(usageInfo["total_tokens"]),
				LatencyMs:    asInt64(usageInfo["latency_ms"]),
				Success:      asBool(usageInfo["success"]),
				ErrorCode:    asString(usageInfo["error_code"]),
				ErrorMessage: asString(usageInfo["error_message"]),
				RequestedAt:  time.Unix(asInt64(usageInfo["requested_at"])/1000, 0),
				CompletedAt:  time.Unix(asInt64(usageInfo["completed_at"])/1000, 0),
			}
			_ = db.Create(usage).Error
		})

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

		utils.Sig().Connect(llm.SignalMessageCreated, func(sender any, params ...any) {
			if len(params) < 1 {
				return
			}
			data, ok := params[0].(llm.MessageCreatedData)
			if !ok {
				return
			}
			msg := &models.ChatMessage{
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
			_ = db.Create(msg).Error
		})
	})
}

func (ch *CinyuHandlers) LLMChat(c *gin.Context) {
	var req llmChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, 400, "invalid request", gin.H{"error": err.Error()})
		return
	}
	input := strings.TrimSpace(req.Input)
	if input == "" {
		response.FailWithCode(c, 400, "input is required", nil)
		return
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = utils.SnowflakeUtil.GenID()
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "anonymous"
	}

	provider := "openai"
	apiKey := ""
	baseURL := "https://api.openai.com/v1"
	model := "gpt-4o-mini"
	maxSessionMessages := 3
	summaryModel := ""
	if config.GlobalConfig != nil {
		llmConf := config.GlobalConfig.Services.LLM
		if strings.TrimSpace(llmConf.Provider) != "" {
			provider = strings.TrimSpace(llmConf.Provider)
		}
		if strings.TrimSpace(llmConf.APIKey) != "" {
			apiKey = strings.TrimSpace(llmConf.APIKey)
		}
		if strings.TrimSpace(llmConf.BaseURL) != "" {
			baseURL = strings.TrimSpace(llmConf.BaseURL)
		}
		if strings.TrimSpace(req.Model) != "" {
			model = strings.TrimSpace(req.Model)
		} else if strings.TrimSpace(llmConf.Model) != "" {
			model = strings.TrimSpace(llmConf.Model)
		}
		if llmConf.MaxSessionMessages > 0 {
			maxSessionMessages = llmConf.MaxSessionMessages
		}
		summaryModel = strings.TrimSpace(llmConf.SummaryModel)
	} else if strings.TrimSpace(req.Model) != "" {
		model = strings.TrimSpace(req.Model)
	}

	h, err := llm.NewLLMProvider(c.Request.Context(), provider, apiKey, baseURL, "")
	if err != nil {
		response.Fail(c, "llm provider init failed", gin.H{"error": err.Error()})
		return
	}

	llm.CreateSession(sessionID, userID, "", h.Provider(), model, "")

	memoryMessages, err := ch.buildSessionMemory(h, sessionID, userID, model, maxSessionMessages, summaryModel)
	if err != nil {
		response.Fail(c, "build session memory failed", gin.H{"error": err.Error()})
		return
	}

	llm.CreateMessage(utils.SnowflakeUtil.GenID(), sessionID, "user", input, 0, model, h.Provider(), "")

	resp, err := h.QueryWithOptions(input, &llm.QueryOptions{
		Model:       model,
		Messages:    memoryMessages,
		SessionID:   sessionID,
		UserID:      userID,
		RequestType: "query",
	})
	if err != nil {
		response.Fail(c, "llm query failed", gin.H{"error": err.Error()})
		return
	}

	output := ""
	if resp != nil && len(resp.Choices) > 0 {
		output = resp.Choices[0].Content
	}
	completionTokens := 0
	if resp != nil && resp.Usage != nil {
		completionTokens = resp.Usage.CompletionTokens
	}
	llm.CreateMessage(utils.SnowflakeUtil.GenID(), sessionID, "assistant", output, completionTokens, model, h.Provider(), resp.RequestID)

	response.Success(c, "ok", llmChatResponse{
		SessionID: sessionID,
		RequestID: resp.RequestID,
		LatencyMs: resp.LatencyMs,
		Output:    output,
		Usage:     resp.Usage,
	})
}

func (ch *CinyuHandlers) LLMChatStream(c *gin.Context) {
	response.FailWithCode(c, http.StatusNotImplemented, "not implemented", nil)
}

func (ch *CinyuHandlers) AgentChatStream(c *gin.Context) {
	var req agentChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithCode(c, 400, "invalid request", gin.H{"error": err.Error()})
		return
	}
	input := strings.TrimSpace(req.Input)
	if input == "" {
		response.FailWithCode(c, 400, "input is required", nil)
		return
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = utils.SnowflakeUtil.GenID()
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "web-user"
	}

	provider := "openai"
	apiKey := ""
	baseURL := "https://api.openai.com/v1"
	model := "qwen-plus"
	fastModel := model
	strongModel := model
	maxSteps := 12
	maxCostTokens := 12000
	maxDuration := 120 * time.Second
	if config.GlobalConfig != nil {
		llmConf := config.GlobalConfig.Services.LLM
		if strings.TrimSpace(llmConf.Provider) != "" {
			provider = strings.TrimSpace(llmConf.Provider)
		}
		if strings.TrimSpace(llmConf.APIKey) != "" {
			apiKey = strings.TrimSpace(llmConf.APIKey)
		}
		if strings.TrimSpace(llmConf.BaseURL) != "" {
			baseURL = strings.TrimSpace(llmConf.BaseURL)
		}
		if strings.TrimSpace(req.Model) != "" {
			model = strings.TrimSpace(req.Model)
		} else if strings.TrimSpace(llmConf.Model) != "" {
			model = strings.TrimSpace(llmConf.Model)
		}
		if strings.TrimSpace(llmConf.AgentFastModel) != "" {
			fastModel = strings.TrimSpace(llmConf.AgentFastModel)
		}
		if strings.TrimSpace(llmConf.AgentStrongModel) != "" {
			strongModel = strings.TrimSpace(llmConf.AgentStrongModel)
		}
		if llmConf.AgentMaxSteps > 0 {
			maxSteps = llmConf.AgentMaxSteps
		}
		if llmConf.AgentMaxCostTokens > 0 {
			maxCostTokens = llmConf.AgentMaxCostTokens
		}
		if llmConf.AgentMaxDurationS > 0 {
			maxDuration = time.Duration(llmConf.AgentMaxDurationS) * time.Second
		}
	}
	if strings.TrimSpace(fastModel) == "" {
		fastModel = model
	}
	if strings.TrimSpace(strongModel) == "" {
		strongModel = model
	}

	h, err := llm.NewLLMProvider(c.Request.Context(), provider, apiKey, baseURL, "")
	if err != nil {
		response.Fail(c, "llm provider init failed", gin.H{"error": err.Error()})
		return
	}
	defer h.Interrupt()
	go func() {
		<-c.Request.Context().Done()
		h.Interrupt()
	}()

	maxTasks := req.MaxTasks
	if maxTasks <= 0 {
		maxTasks = 6
	}

	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.FailWithCode(c, 500, "streaming unsupported", nil)
		return
	}

	writeEvt := func(event string, payload any) bool {
		line := map[string]any{
			"event": event,
			"data":  payload,
		}
		b, _ := json.Marshal(line)
		if _, err := c.Writer.Write(append(b, '\n')); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	runtimeCfg := agentRuntimeConfig{
		FastModel:     fastModel,
		StrongModel:   strongModel,
		MaxTasks:      maxTasks,
		MaxSteps:      maxSteps,
		MaxCostTokens: maxCostTokens,
		MaxDuration:   maxDuration,
	}
	runID, finalText, runErr := ch.runAgentRuntime(c.Request.Context(), h, sessionID, userID, input, runtimeCfg, writeEvt)
	if finalText == "" {
		finalText = "Agent 执行完成，但没有可展示的输出。"
	}
	llm.CreateSession(sessionID, userID, "Agent会话", h.Provider(), model, "")
	llm.CreateMessage(utils.SnowflakeUtil.GenID(), sessionID, "user", input, 0, model, h.Provider(), "")
	llm.CreateMessage(utils.SnowflakeUtil.GenID(), sessionID, "assistant", finalText, 0, model, h.Provider(), "")
	_ = writeEvt("final", gin.H{"session_id": sessionID, "run_id": runID, "output": finalText, "error": errString(runErr)})
	_ = writeEvt("done", gin.H{"ok": runErr == nil, "run_id": runID})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (ch *CinyuHandlers) buildSessionMemory(h llm.LLMHandler, sessionID, userID, model string, maxSessionMessages int, summaryModel string) ([]llm.ChatMessage, error) {
	var dbMessages []models.ChatMessage
	if err := ch.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&dbMessages).Error; err != nil {
		return nil, err
	}
	if maxSessionMessages <= 0 {
		maxSessionMessages = 10
	}
	if len(dbMessages) > maxSessionMessages {
		compact, err := ch.compactSessionMessages(h, sessionID, userID, model, summaryModel, dbMessages)
		if err != nil {
			return nil, err
		}
		dbMessages = compact
	}
	out := make([]llm.ChatMessage, 0, len(dbMessages))
	for _, m := range dbMessages {
		role := strings.TrimSpace(strings.ToLower(m.Role))
		if role != "user" && role != "assistant" && role != "system" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		out = append(out, llm.ChatMessage{Role: role, Content: content})
	}
	return out, nil
}

func (ch *CinyuHandlers) compactSessionMessages(h llm.LLMHandler, sessionID, userID, model, summaryModel string, dbMessages []models.ChatMessage) ([]models.ChatMessage, error) {
	transcript := make([]string, 0, len(dbMessages))
	for _, m := range dbMessages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "" {
			role = "user"
		}
		transcript = append(transcript, fmt.Sprintf("%s: %s", role, content))
	}
	prompt := "请把下面的历史会话压缩成一条短期记忆摘要，保留：用户偏好、关键事实、未完成任务、重要上下文。输出纯文本，不要markdown。\n\n" + strings.Join(transcript, "\n")
	if strings.TrimSpace(summaryModel) == "" {
		summaryModel = model
	}
	summaryResp, err := h.QueryWithOptions(prompt, &llm.QueryOptions{
		Model:       summaryModel,
		SessionID:   sessionID,
		UserID:      userID,
		RequestType: "summary",
	})
	if err != nil {
		return nil, err
	}
	summary := ""
	if summaryResp != nil && len(summaryResp.Choices) > 0 {
		summary = strings.TrimSpace(summaryResp.Choices[0].Content)
	}
	if summary == "" {
		return nil, fmt.Errorf("empty summary response")
	}
	summaryMessage := models.ChatMessage{
		ID:         utils.SnowflakeUtil.GenID(),
		SessionID:  sessionID,
		Role:       "system",
		Content:    "会话摘要: " + summary,
		TokenCount: 0,
		Model:      summaryModel,
		Provider:   h.Provider(),
		RequestID:  summaryResp.RequestID,
	}
	if err := ch.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&summaryMessage).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return []models.ChatMessage{summaryMessage}, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	if ok {
		return b
	}
	return false
}

func asInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

func asInt(v any) int {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}
