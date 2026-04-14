package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	agentplan "github.com/LingByte/lingoroutine/agent/plan"
	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/models"
	"github.com/LingByte/lingoroutine/utils"
)

const (
	agentStatusQueued      = "queued"
	agentStatusRunning     = "running"
	agentStatusWaitingTool = "waiting_tool"
	agentStatusSucceeded   = "succeeded"
	agentStatusFailed      = "failed"
	agentStatusCancelled   = "cancelled"
)

type agentStreamWriter func(event string, payload any) bool

type agentRuntimeConfig struct {
	FastModel     string
	StrongModel   string
	MaxTasks      int
	MaxSteps      int
	MaxCostTokens int
	MaxDuration   time.Duration
}

func (ch *CinyuHandlers) runAgentRuntime(
	ctx context.Context,
	h llm.LLMHandler,
	sessionID, userID, goal string,
	cfg agentRuntimeConfig,
	write agentStreamWriter,
) (string, string, error) {
	now := time.Now()
	run := &models.AgentRun{
		ID:            utils.SnowflakeUtil.GenID(),
		SessionID:     sessionID,
		UserID:        userID,
		Goal:          goal,
		Status:        agentStatusQueued,
		Phase:         "planning",
		MaxSteps:      cfg.MaxSteps,
		MaxCostTokens: cfg.MaxCostTokens,
		MaxDurationMs: cfg.MaxDuration.Milliseconds(),
		StartedAt:     now,
	}
	if err := ch.db.Create(run).Error; err != nil {
		return "", "", err
	}
	_ = write("run", map[string]any{"run_id": run.ID, "status": run.Status, "phase": run.Phase})

	run.Status = agentStatusRunning
	if err := ch.db.Save(run).Error; err != nil {
		return run.ID, "", err
	}
	if !write("status", map[string]any{"phase": "planning", "message": "正在拆解任务..."}) {
		return run.ID, "", context.Canceled
	}

	decomposer := &agentplan.LLMDecomposer{
		LLM:      h,
		Model:    cfg.FastModel,
		MaxTasks: cfg.MaxTasks,
	}
	p, err := decomposer.Decompose(ctx, agentplan.Request{
		Goal:     goal,
		MaxTasks: cfg.MaxTasks,
		LLMModel: cfg.FastModel,
	})
	if err != nil {
		run.Status = agentStatusFailed
		run.ErrorMessage = err.Error()
		run.CompletedAt = time.Now()
		_ = ch.db.Save(run).Error
		return run.ID, "", err
	}
	if b, e := json.Marshal(p); e == nil {
		run.PlanJSON = string(b)
		_ = ch.db.Save(run).Error
	}
	if !write("plan", map[string]any{"goal": p.Goal, "tasks": p.Tasks, "task_count": len(p.Tasks)}) {
		return run.ID, "", context.Canceled
	}
	run.Phase = "executing"
	_ = ch.db.Save(run).Error

	remaining := make([]agentplan.Task, len(p.Tasks))
	copy(remaining, p.Tasks)
	outputs := map[string]string{}
	totalTokens := 0
	stepNum := 0
	finalText := ""
	started := time.Now()

	for len(remaining) > 0 {
		if err := ctx.Err(); err != nil {
			run.Status = agentStatusCancelled
			run.ErrorMessage = "cancelled"
			run.CompletedAt = time.Now()
			run.TotalSteps = stepNum
			run.TotalTokens = totalTokens
			_ = ch.db.Save(run).Error
			return run.ID, finalText, err
		}
		if cfg.MaxSteps > 0 && stepNum >= cfg.MaxSteps {
			err := fmt.Errorf("max_steps exceeded: %d", cfg.MaxSteps)
			run.Status = agentStatusFailed
			run.ErrorMessage = err.Error()
			run.CompletedAt = time.Now()
			run.TotalSteps = stepNum
			run.TotalTokens = totalTokens
			_ = ch.db.Save(run).Error
			return run.ID, finalText, err
		}
		if cfg.MaxDuration > 0 && time.Since(started) > cfg.MaxDuration {
			err := fmt.Errorf("max_duration exceeded: %s", cfg.MaxDuration)
			run.Status = agentStatusFailed
			run.ErrorMessage = err.Error()
			run.CompletedAt = time.Now()
			run.TotalSteps = stepNum
			run.TotalTokens = totalTokens
			_ = ch.db.Save(run).Error
			return run.ID, finalText, err
		}
		if cfg.MaxCostTokens > 0 && totalTokens >= cfg.MaxCostTokens {
			err := fmt.Errorf("max_cost_tokens exceeded: %d", cfg.MaxCostTokens)
			run.Status = agentStatusFailed
			run.ErrorMessage = err.Error()
			run.CompletedAt = time.Now()
			run.TotalSteps = stepNum
			run.TotalTokens = totalTokens
			_ = ch.db.Save(run).Error
			return run.ID, finalText, err
		}

		task := remaining[0]
		remaining = remaining[1:]
		stepNum++
		stepID := fmt.Sprintf("step-%03d", stepNum)
		inRaw, _ := json.Marshal(task.Input)
		step := &models.AgentStep{
			ID:          utils.SnowflakeUtil.GenID(),
			RunID:       run.ID,
			StepID:      stepID,
			TaskID:      task.ID,
			Title:       task.Title,
			Instruction: task.Instruction,
			Status:      agentStatusQueued,
			Model:       cfg.StrongModel,
			InputJSON:   string(inRaw),
		}
		if err := ch.db.Create(step).Error; err != nil {
			return run.ID, finalText, err
		}
		_ = write("step", map[string]any{"run_id": run.ID, "step_id": stepID, "task_id": task.ID, "status": agentStatusQueued})

		step.Status = agentStatusRunning
		step.StartedAt = time.Now()
		_ = ch.db.Save(step).Error
		_ = write("step", map[string]any{"run_id": run.ID, "step_id": stepID, "task_id": task.ID, "status": agentStatusRunning, "index": stepNum})

		var out string
		var usage *llm.TokenUsage
		var stepErr error
		if tool := strings.TrimSpace(anyToString(task.Input["tool"])); tool != "" {
			if err := validateToolInvocation(tool, task.Input); err != nil {
				stepErr = err
			}
			step.Status = agentStatusWaitingTool
			_ = ch.db.Save(step).Error
			_ = write("step", map[string]any{"run_id": run.ID, "step_id": stepID, "task_id": task.ID, "status": agentStatusWaitingTool, "tool": tool})
			if stepErr == nil {
				out, stepErr = runToolWithContext(ctx, tool, task.Input)
			}
		} else {
			out, usage, stepErr = ch.runTaskWithFallback(ctx, h, cfg, goal, task, outputs)
		}

		if usage != nil {
			step.InputTokens = usage.PromptTokens
			step.OutputTokens = usage.CompletionTokens
			step.TotalTokens = usage.TotalTokens
			totalTokens += usage.TotalTokens
		}
		if stepErr != nil {
			step.Status = agentStatusFailed
			step.ErrorMessage = stepErr.Error()
			step.CompletedAt = time.Now()
			step.LatencyMs = step.CompletedAt.Sub(step.StartedAt).Milliseconds()
			_ = ch.db.Save(step).Error
			_ = write("step", map[string]any{
				"run_id": run.ID, "step_id": stepID, "task_id": task.ID, "status": step.Status,
				"error": step.ErrorMessage, "latency_ms": step.LatencyMs, "index": stepNum,
			})

			newTasks, replanReason := ch.replanAfterFailure(ctx, h, cfg, goal, task, outputs, stepErr.Error())
			if len(newTasks) > 0 {
				run.Phase = "reflecting"
				_ = ch.db.Save(run).Error
				remaining = append(newTasks, remaining...)
				_ = write("status", map[string]any{"phase": "reflecting", "message": "检测到失败，已动态重规划", "reason": replanReason, "new_tasks": len(newTasks)})
				run.Phase = "executing"
				_ = ch.db.Save(run).Error
				continue
			}
			run.Status = agentStatusFailed
			run.ErrorMessage = stepErr.Error()
			run.CompletedAt = time.Now()
			run.TotalSteps = stepNum
			run.TotalTokens = totalTokens
			_ = ch.db.Save(run).Error
			return run.ID, finalText, stepErr
		}

		step.Status = agentStatusSucceeded
		step.OutputText = strings.TrimSpace(out)
		step.CompletedAt = time.Now()
		step.LatencyMs = step.CompletedAt.Sub(step.StartedAt).Milliseconds()
		_ = ch.db.Save(step).Error
		outputs[task.ID] = step.OutputText
		finalText = step.OutputText
		_ = write("step", map[string]any{
			"run_id": run.ID, "step_id": stepID, "task_id": task.ID, "status": step.Status,
			"output": step.OutputText, "latency_ms": step.LatencyMs, "index": stepNum,
			"input_tokens": step.InputTokens, "output_tokens": step.OutputTokens, "total_tokens": step.TotalTokens,
		})

		reflectTasks, reflectReason := ch.reflectAndReplan(ctx, h, cfg, goal, outputs, remaining)
		if len(reflectTasks) > 0 {
			run.Phase = "reflecting"
			_ = ch.db.Save(run).Error
			remaining = reflectTasks
			_ = write("status", map[string]any{"phase": "reflecting", "message": "已根据执行结果动态调整计划", "reason": reflectReason, "task_count": len(reflectTasks)})
			run.Phase = "executing"
			_ = ch.db.Save(run).Error
		}
	}

	run.Status = agentStatusSucceeded
	run.Phase = "done"
	run.ResultText = strings.TrimSpace(finalText)
	run.TotalSteps = stepNum
	run.TotalTokens = totalTokens
	run.CompletedAt = time.Now()
	_ = ch.db.Save(run).Error
	return run.ID, run.ResultText, nil
}

func (ch *CinyuHandlers) runTaskWithFallback(
	ctx context.Context,
	h llm.LLMHandler,
	cfg agentRuntimeConfig,
	goal string,
	task agentplan.Task,
	outputs map[string]string,
) (string, *llm.TokenUsage, error) {
	prompt := buildTaskExecutionPrompt(goal, task, outputs, "")
	resp, err := h.QueryWithOptions(prompt, &llm.QueryOptions{Model: cfg.StrongModel, RequestType: "agent_step"})
	if err == nil && resp != nil && len(resp.Choices) > 0 {
		out := strings.TrimSpace(resp.Choices[0].Content)
		ok, feedback := ch.evaluateTask(ctx, h, cfg, goal, task, out)
		if ok {
			return out, resp.Usage, nil
		}
		prompt = buildTaskExecutionPrompt(goal, task, outputs, feedback+"\n请缩小范围给出最小可行输出。")
	}
	resp2, err2 := h.QueryWithOptions(prompt, &llm.QueryOptions{Model: cfg.FastModel, RequestType: "agent_step_fallback"})
	if err2 != nil {
		if err != nil {
			return "", nil, err
		}
		return "", nil, err2
	}
	if resp2 == nil || len(resp2.Choices) == 0 {
		return "", resp2.Usage, fmt.Errorf("empty fallback output")
	}
	return strings.TrimSpace(resp2.Choices[0].Content), resp2.Usage, nil
}

func (ch *CinyuHandlers) evaluateTask(
	ctx context.Context,
	h llm.LLMHandler,
	cfg agentRuntimeConfig,
	goal string,
	task agentplan.Task,
	output string,
) (bool, string) {
	_ = ctx
	if strings.TrimSpace(task.Expected) == "" {
		return true, ""
	}
	prompt := "你是任务验收器。只输出JSON：{\"ok\":bool,\"feedback\":string}。\n" +
		"goal: " + goal + "\n" +
		"task: " + task.Title + "\n" +
		"expected: " + task.Expected + "\n" +
		"output:\n" + output
	resp, err := h.QueryWithOptions(prompt, &llm.QueryOptions{Model: cfg.StrongModel, RequestType: "agent_eval"})
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return true, ""
	}
	raw := strings.TrimSpace(resp.Choices[0].Content)
	type eval struct {
		OK       bool   `json:"ok"`
		Feedback string `json:"feedback"`
	}
	var e eval
	if j := agentplan.ExtractJSON(raw); strings.TrimSpace(j) != "" {
		raw = j
	}
	if json.Unmarshal([]byte(raw), &e) != nil {
		return true, ""
	}
	return e.OK, strings.TrimSpace(e.Feedback)
}

func (ch *CinyuHandlers) replanAfterFailure(
	ctx context.Context,
	h llm.LLMHandler,
	cfg agentRuntimeConfig,
	goal string,
	failed agentplan.Task,
	outputs map[string]string,
	reason string,
) ([]agentplan.Task, string) {
	_ = ctx
	prompt := "你是重规划器。某任务失败，请给出替代后续任务。\n" +
		"只输出JSON: {\"reason\":string,\"tasks\":[{\"id\":string,\"title\":string,\"instruction\":string,\"expected\":string,\"depends_on\":[string],\"can_parallel\":bool,\"input\":object}]}\n" +
		"goal: " + goal + "\n" +
		"failed_task: " + failed.Title + "\n" +
		"failure_reason: " + reason + "\n" +
		"已有输出: " + mapToText(outputs)
	resp, err := h.QueryWithOptions(prompt, &llm.QueryOptions{Model: cfg.FastModel, RequestType: "agent_replan"})
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return nil, ""
	}
	raw := strings.TrimSpace(resp.Choices[0].Content)
	if j := agentplan.ExtractJSON(raw); strings.TrimSpace(j) != "" {
		raw = j
	}
	var out struct {
		Reason string           `json:"reason"`
		Tasks  []agentplan.Task `json:"tasks"`
	}
	if json.Unmarshal([]byte(raw), &out) != nil || len(out.Tasks) == 0 {
		return nil, ""
	}
	return out.Tasks, strings.TrimSpace(out.Reason)
}

func (ch *CinyuHandlers) reflectAndReplan(
	ctx context.Context,
	h llm.LLMHandler,
	cfg agentRuntimeConfig,
	goal string,
	outputs map[string]string,
	remaining []agentplan.Task,
) ([]agentplan.Task, string) {
	_ = ctx
	if len(remaining) == 0 {
		return nil, ""
	}
	prompt := "你是反思器。根据当前产出判断是否需要重排后续任务。\n" +
		"只输出JSON: {\"replan\":bool,\"reason\":string,\"tasks\":[task]}\n" +
		"goal: " + goal + "\n" +
		"current_outputs: " + mapToText(outputs) + "\n" +
		"remaining_task_count: " + fmt.Sprintf("%d", len(remaining))
	resp, err := h.QueryWithOptions(prompt, &llm.QueryOptions{Model: cfg.FastModel, RequestType: "agent_reflect"})
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return nil, ""
	}
	raw := strings.TrimSpace(resp.Choices[0].Content)
	if j := agentplan.ExtractJSON(raw); strings.TrimSpace(j) != "" {
		raw = j
	}
	var out struct {
		Replan bool             `json:"replan"`
		Reason string           `json:"reason"`
		Tasks  []agentplan.Task `json:"tasks"`
	}
	if json.Unmarshal([]byte(raw), &out) != nil || !out.Replan || len(out.Tasks) == 0 {
		return nil, ""
	}
	return out.Tasks, strings.TrimSpace(out.Reason)
}

func buildTaskExecutionPrompt(goal string, task agentplan.Task, outputs map[string]string, extra string) string {
	b := strings.Builder{}
	b.WriteString("你是任务执行器。\n")
	b.WriteString("总目标: " + strings.TrimSpace(goal) + "\n")
	b.WriteString("任务: " + strings.TrimSpace(task.Title) + "\n")
	b.WriteString("指令: " + strings.TrimSpace(task.Instruction) + "\n")
	if strings.TrimSpace(task.Expected) != "" {
		b.WriteString("验收标准: " + strings.TrimSpace(task.Expected) + "\n")
	}
	if len(task.DependsOn) > 0 {
		b.WriteString("上游输出:\n")
		for _, dep := range task.DependsOn {
			if v := strings.TrimSpace(outputs[dep]); v != "" {
				b.WriteString("- [" + dep + "] " + v + "\n")
			}
		}
	}
	if strings.TrimSpace(extra) != "" {
		b.WriteString("附加要求: " + strings.TrimSpace(extra) + "\n")
	}
	b.WriteString("请直接输出结果，不要解释。")
	return b.String()
}

func runToolWithContext(ctx context.Context, tool string, input map[string]any) (string, error) {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "echo":
		return anyToString(input["text"]), nil
	case "sleep":
		sec := anyToInt(input["seconds"])
		if sec <= 0 {
			sec = 1
		}
		timer := time.NewTimer(time.Duration(sec) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return fmt.Sprintf("slept %ds", sec), nil
		}
	case "http_get":
		url := strings.TrimSpace(anyToString(input["url"]))
		if url == "" {
			return "", fmt.Errorf("tool http_get requires url")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		out := strings.TrimSpace(string(raw))
		if len(out) > 2000 {
			out = out[:2000]
		}
		return out, nil
	default:
		return "", fmt.Errorf("unsupported tool: %s", tool)
	}
}

func validateToolInvocation(tool string, input map[string]any) error {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "echo":
		if strings.TrimSpace(anyToString(input["text"])) == "" {
			return fmt.Errorf("tool echo requires text")
		}
		return nil
	case "sleep":
		sec := anyToInt(input["seconds"])
		if sec <= 0 || sec > 30 {
			return fmt.Errorf("tool sleep seconds must be in (0,30]")
		}
		return nil
	case "http_get":
		raw := strings.TrimSpace(anyToString(input["url"]))
		if raw == "" {
			return fmt.Errorf("tool http_get requires url")
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("tool http_get invalid url")
		}
		if u.Scheme != "https" {
			return fmt.Errorf("tool http_get only supports https")
		}
		host := strings.ToLower(u.Hostname())
		if host == "localhost" {
			return fmt.Errorf("tool http_get localhost is blocked")
		}
		ip := net.ParseIP(host)
		if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
			return fmt.Errorf("tool http_get private ip is blocked")
		}
		allow := map[string]bool{
			"api.open-meteo.com": true,
			"wttr.in":            true,
		}
		if !allow[host] {
			return fmt.Errorf("tool http_get host not allowed: %s", host)
		}
		return nil
	default:
		return fmt.Errorf("unsupported tool: %s", tool)
	}
}

func mapToText(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	b := strings.Builder{}
	for k, v := range m {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(v))
		b.WriteString("; ")
	}
	return b.String()
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func anyToInt(v any) int {
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
