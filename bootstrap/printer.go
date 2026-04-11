package bootstrap

// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

import (
	"fmt"
	"os"
	"strings"
)

// PrintBannerFromFile Read file and print
func (b *Bootstrap) PrintBannerFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	colors := []string{
		"\x1b[38;5;165m",
		"\x1b[38;5;189m",
		"\x1b[38;5;207m",
		"\x1b[38;5;219m",
		"\x1b[38;5;225m",
		"\x1b[38;5;231m",
	}

	for i, line := range lines {
		color := colors[i%len(colors)]
		fmt.Println(color + line + "\x1b[0m")
	}
	return nil
}
