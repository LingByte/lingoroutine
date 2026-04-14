package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/LingByte/lingoroutine/agent/exec"
	"github.com/LingByte/lingoroutine/agent/plan"
	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/utils"
)

// StreamingTaskRunner 支持流式输出的任务执行器
type StreamingTaskRunner struct {
	LLM   llm.LLMHandler
	Model string
}

func (r *StreamingTaskRunner) RunTask(ctx context.Context, task plan.Task, st *exec.State) (string, error) {
	if r == nil || r.LLM == nil {
		return "", exec.ErrMissingRunner
	}

	model := strings.TrimSpace(r.Model)
	if model == "" {
		model = utils.GetEnv("LLM_MODEL")
	}

	// 构建提示词
	b := strings.Builder{}
	b.WriteString("你是一个任务执行器。根据目标和当前任务指令完成任务。\n")
	b.WriteString("约束：如果任务需要依赖上游任务输出，请仅使用提供的上游输出；不要编造不存在的信息。\n\n")
	b.WriteString("总目标: ")
	b.WriteString(strings.TrimSpace(st.Goal))
	b.WriteString("\n")
	b.WriteString("当前任务: ")
	b.WriteString(strings.TrimSpace(task.Title))
	b.WriteString("\n")
	b.WriteString("任务指令: ")
	b.WriteString(strings.TrimSpace(task.Instruction))
	b.WriteString("\n\n")
	if len(task.DependsOn) > 0 {
		b.WriteString("上游输出：\n")
		for _, dep := range task.DependsOn {
			if dep == "" {
				continue
			}
			b.WriteString("[")
			b.WriteString(dep)
			b.WriteString("] ")
			b.WriteString(strings.TrimSpace(st.Outputs[dep]))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if st != nil && st.Feedback != nil {
		if fb := strings.TrimSpace(st.Feedback[task.ID]); fb != "" {
			b.WriteString("验收反馈（上一轮未通过原因）：\n")
			b.WriteString(fb)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("请输出该任务的结果：\n")

	// 使用流式查询
	var fullOutput strings.Builder
	_, err := r.LLM.QueryStream(b.String(), &llm.QueryOptions{Model: model}, func(segment string, isComplete bool) error {
		fmt.Printf("[%s] %s", task.Title, segment)
		fullOutput.WriteString(segment)
		if isComplete {
			fmt.Println() // 换行
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fullOutput.String()), nil
}

func main() {
	ctx := context.Background()

	provider, err := llm.NewLLMProvider(ctx,
		utils.GetEnv("LLM_PROVIDER"),
		utils.GetEnv("LLM_API_KEY"),
		utils.GetEnv("LLM_BASEURL"),
		"",
	)
	if err != nil {
		log.Fatalf("创建 LLM handler 失败: %v", err)
	}

	// 1. 任务分解
	fmt.Println("任务分解中...")
	decomposer := &plan.LLMDecomposer{
		LLM:      provider,
		Model:    utils.GetEnv("LLM_MODEL"),
		MaxTasks: 3,
	}

	planReq := plan.Request{
		Goal:     "你是谁",
		MaxTasks: 3,
		LLMModel: utils.GetEnv("LLM_MODEL"),
	}

	taskPlan, err := decomposer.Decompose(ctx, planReq)
	if err != nil {
		log.Fatalf("任务分解失败: %v", err)
	}

	fmt.Printf("分解得到 %d 个任务:\n", len(taskPlan.Tasks))
	for i, task := range taskPlan.Tasks {
		fmt.Printf("  %d. %s\n", i+1, task.Title)
	}

	// 2. 任务执行
	fmt.Println("任务执行中...")
	runner := &StreamingTaskRunner{
		LLM:   provider,
		Model: utils.GetEnv("LLM_MODEL"),
	}

	evaluator := &exec.LLMTaskEvaluator{
		LLM:   provider,
		Model: utils.GetEnv("LLM_MODEL"),
	}

	executor := &exec.Executor{
		Runner:    runner,
		Evaluator: evaluator,
		Opts: exec.Options{
			StopOnError: false,
			MaxTasks:    10,
			MaxAttempts: 2,
		},
	}

	result, err := executor.Run(ctx, taskPlan)
	if err != nil {
		log.Fatalf("任务执行失败: %v", err)
	}

	// 3. 显示结果
	fmt.Println("\n✅ 执行完成!")
	fmt.Println("\n📊 执行统计:")
	for _, taskResult := range result.TaskResults {
		fmt.Printf("任务 %s: %s (尝试 %d 次)\n", taskResult.TaskID, taskResult.Status, taskResult.Attempts)
	}
	fmt.Println("---")
}
