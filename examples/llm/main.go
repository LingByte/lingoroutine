package main

import (
	"context"
	"fmt"

	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/utils"
)

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

func main() {
	ctx := context.Background()

	provider, err := llm.NewLLMProvider(ctx,
		utils.GetEnv("LLM_PROVIDER"),
		utils.GetEnv("LLM_API_KEY"),
		utils.GetEnv("LLM_BASEURL"),
		"",
	)
	if err != nil {
		panic(err)
	}
	query, err := provider.Query("你好", utils.GetEnv("LLM_MODEL"))
	if err != nil {
		panic(err)
	}
	fmt.Println(query)
}
