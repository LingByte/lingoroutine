package handlers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeLLM struct {
	queryFn            func(text, model string) (string, error)
	queryWithOptionsFn func(text string, options *llm.QueryOptions) (*llm.QueryResponse, error)
}

func (f *fakeLLM) Query(text, model string) (string, error) {
	if f.queryFn != nil {
		return f.queryFn(text, model)
	}
	return "", nil
}
func (f *fakeLLM) QueryWithOptions(text string, options *llm.QueryOptions) (*llm.QueryResponse, error) {
	if f.queryWithOptionsFn != nil {
		return f.queryWithOptionsFn(text, options)
	}
	return nil, nil
}
func (f *fakeLLM) QueryStream(text string, options *llm.QueryOptions, callback func(segment string, isComplete bool) error) (*llm.QueryResponse, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeLLM) Provider() string { return "openai" }
func (f *fakeLLM) Interrupt()       {}

func newTestHandlers(t *testing.T) *CinyuHandlers {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentRun{}, &models.AgentStep{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &CinyuHandlers{db: db}
}

func TestRunAgentRuntime_MaxStepsExceeded(t *testing.T) {
	ch := newTestHandlers(t)
	h := &fakeLLM{
		queryFn: func(text, model string) (string, error) {
			return `{"goal":"g","tasks":[{"id":"t1","title":"a","instruction":"do a"},{"id":"t2","title":"b","instruction":"do b"}]}`, nil
		},
		queryWithOptionsFn: func(text string, options *llm.QueryOptions) (*llm.QueryResponse, error) {
			return &llm.QueryResponse{
				Choices: []llm.QueryChoice{{Content: "ok"}},
				Usage:   &llm.TokenUsage{TotalTokens: 10},
			}, nil
		},
	}
	_, _, err := ch.runAgentRuntime(context.Background(), h, "s1", "u1", "goal", agentRuntimeConfig{
		FastModel: "f", StrongModel: "s", MaxTasks: 4, MaxSteps: 1, MaxCostTokens: 1000, MaxDuration: time.Minute,
	}, func(string, any) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "max_steps exceeded") {
		t.Fatalf("expected max_steps exceeded, got: %v", err)
	}
}

func TestRunAgentRuntime_Cancelled(t *testing.T) {
	ch := newTestHandlers(t)
	h := &fakeLLM{
		queryFn: func(text, model string) (string, error) {
			return `{"goal":"g","tasks":[{"id":"t1","title":"a","instruction":"do a"}]}`, nil
		},
		queryWithOptionsFn: func(text string, options *llm.QueryOptions) (*llm.QueryResponse, error) {
			return &llm.QueryResponse{
				Choices: []llm.QueryChoice{{Content: "ok"}},
				Usage:   &llm.TokenUsage{TotalTokens: 1},
			}, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := ch.runAgentRuntime(ctx, h, "s2", "u2", "goal", agentRuntimeConfig{
		FastModel: "f", StrongModel: "s", MaxTasks: 4, MaxSteps: 4, MaxCostTokens: 1000, MaxDuration: time.Minute,
	}, func(string, any) bool { return true })
	if err == nil {
		t.Fatalf("expected cancel error")
	}
}

func TestRunAgentRuntime_ReplanAfterFailure(t *testing.T) {
	ch := newTestHandlers(t)
	h := &fakeLLM{
		queryFn: func(text, model string) (string, error) {
			// first plan contains unsupported tool to force failure.
			return `{"goal":"g","tasks":[{"id":"t1","title":"tool","instruction":"bad tool","input":{"tool":"bad_tool"}}]}`, nil
		},
		queryWithOptionsFn: func(text string, options *llm.QueryOptions) (*llm.QueryResponse, error) {
			switch options.RequestType {
			case "agent_replan":
				return &llm.QueryResponse{
					Choices: []llm.QueryChoice{{Content: `{"reason":"fallback","tasks":[{"id":"t2","title":"retry","instruction":"do retry"}]}`}},
				}, nil
			case "agent_reflect":
				return &llm.QueryResponse{
					Choices: []llm.QueryChoice{{Content: `{"replan":false}`}},
				}, nil
			case "agent_step", "agent_step_fallback":
				return &llm.QueryResponse{
					Choices: []llm.QueryChoice{{Content: "final output"}},
					Usage:   &llm.TokenUsage{TotalTokens: 5},
				}, nil
			case "agent_eval":
				return &llm.QueryResponse{
					Choices: []llm.QueryChoice{{Content: `{"ok":true,"feedback":""}`}},
				}, nil
			default:
				return &llm.QueryResponse{Choices: []llm.QueryChoice{{Content: "ok"}}}, nil
			}
		},
	}
	runID, final, err := ch.runAgentRuntime(context.Background(), h, "s3", "u3", "goal", agentRuntimeConfig{
		FastModel: "f", StrongModel: "s", MaxTasks: 4, MaxSteps: 10, MaxCostTokens: 1000, MaxDuration: time.Minute,
	}, func(string, any) bool { return true })
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if strings.TrimSpace(final) == "" || strings.TrimSpace(runID) == "" {
		t.Fatalf("expected run id and final output")
	}
	var run models.AgentRun
	if e := ch.db.Where("id = ?", runID).First(&run).Error; e != nil {
		t.Fatalf("query run: %v", e)
	}
	if run.Status != agentStatusSucceeded {
		t.Fatalf("expected run succeeded, got %s", run.Status)
	}
	var steps []models.AgentStep
	if e := ch.db.Where("run_id = ?", runID).Order("created_at asc").Find(&steps).Error; e != nil {
		t.Fatalf("query steps: %v", e)
	}
	if len(steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(steps))
	}
}
