package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/LingByte/lingoroutine/parser"
)

func main() {
	ctx := context.Background()
	opts := &parser.ParseOptions{
		MaxTextLength:      10000,
		IncludeTables:      true,
		IncludeHidden:      false,
		PreserveLineBreaks: true,
	}

	// 要解析的文件列表
	files := []string{
		"examples/parser_demo/samples/sample.txt",
		"examples/parser_demo/samples/sample.md",
		"examples/parser_demo/samples/sample.csv",
		"examples/parser_demo/samples/sample.json",
		"examples/parser_demo/samples/sample.yaml",
		"examples/parser_demo/samples/sample.html",
		"examples/parser_demo/samples/sample.eml",
		"examples/parser_demo/samples/sample.rtf",
	}

	for _, filename := range files {
		filePath := filepath.Join("examples/parser_demo/samples", filename)
		fmt.Printf("正在解析文件: %s\n", filename)
		fmt.Printf("文件路径: %s\n", filePath)

		// 解析文件
		result, err := parser.ParsePath(ctx, filePath, opts)
		if err != nil {
			fmt.Printf("解析失败: %v\n\n", err)
			continue
		}

		// 显示解析结果
		fmt.Printf("文件类型: %s\n", result.FileType)
		fmt.Printf("文件名: %s\n", result.FileName)
		fmt.Printf("解析时间: %s\n", result.ParsedAt.Format(time.RFC3339))
		fmt.Printf("文本长度: %d 字符\n", len(result.Text))
		fmt.Printf("分段数量: %d\n", len(result.Sections))

		if len(result.Metadata) > 0 {
			fmt.Printf("元数据: %v\n", result.Metadata)
		}

		// 显示前200个字符的文本内容
		if len(result.Text) > 0 {
			preview := result.Text
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Printf("内容预览: %s\n", preview)
		}

		// 显示分段信息
		if len(result.Sections) > 1 {
			fmt.Printf("分段信息:\n")
			for i, section := range result.Sections {
				fmt.Printf("  分段 %d: 类型=%s, 标题=%s, 长度=%d\n",
					i+1, section.Type, section.Title, len(section.Text))
			}
		}

		fmt.Println(strings.Repeat("-", 80))
		fmt.Println()
	}

	jsonContent := `{"name": "测试", "age": 25, "city": "北京"}`
	result, err := parser.ParseBytes(ctx, "test.json", []byte(jsonContent), opts)
	if err != nil {
		fmt.Printf("字节数组解析失败: %v\n", err)
	} else {
		fmt.Printf("字节数组解析成功!\n")
		fmt.Printf("文件类型: %s\n", result.FileType)
		fmt.Printf("内容: %s\n", result.Text)
	}
}
