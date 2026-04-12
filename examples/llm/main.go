package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	model := utils.GetEnv("LLM_MODEL")
	question := strings.TrimSpace(utils.GetEnv("LLM_QUESTION"))
	if question == "" {
		question = "去年上海发布的白皮书有哪些"
	}
	allowed := []string{"source", "doc_type", "namespace", "location", "years", "dates", "tags_any"}
	ex := llm.NewSelfQueryExtractor(provider, allowed)
	opt := &llm.SelfQueryOptions{Model: model}
	opt.UsePlainQuery = true
	res, err := ex.Extract(ctx, question, opt)
	if err != nil {
		panic(err)
	}
	enc, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(enc))
	//resp, err := provider.QueryWithOptions("那个啥，就是想问下上海去年白皮书", &llm.QueryOptions{
	//	Model:                     "gpt-4o-mini",
	//	EnableQueryRewrite:        true,
	//	QueryRewriteInstruction:   "保留时间地点实体",
	//	EnableQueryExpansion:      true,
	//})
}
