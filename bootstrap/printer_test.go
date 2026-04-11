package bootstrap

// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LingByte/lingoroutine/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	logger.Init(&logger.LogConfig{
		Level:    "info",
		Filename: "",
		MaxSize:  100,
		MaxAge:   30,
		MaxBackups: 3,
	}, "test")
}

func TestPrintBannerFromFile(t *testing.T) {
	// Create temporary banner file
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "banner.txt")

	bannerContent := `
  ╔══════════════════════════════════════╗
  ║            Test Banner               ║
  ║         Welcome to LingByte          ║
  ╚══════════════════════════════════════╝
`
	err := os.WriteFile(bannerPath, []byte(bannerContent), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call function
	err = bs.PrintBannerFromFile(bannerPath)
	assert.NoError(t, err)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(r)
	output := outputBuf.String()

	// Verify output contains banner content (without ANSI codes)
	assert.Contains(t, output, "Test Banner")
	assert.Contains(t, output, "Welcome to LingByte")

	// Verify ANSI color codes are present
	assert.Contains(t, output, "\x1b[38;5;")
	assert.Contains(t, output, "\x1b[0m")
}

func TestPrintBannerFromFile_FileNotFound(t *testing.T) {
	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)
	err := bs.PrintBannerFromFile("/nonexistent/banner.txt")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestPrintBannerFromFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "empty.txt")

	err := os.WriteFile(bannerPath, []byte(""), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = bs.PrintBannerFromFile(bannerPath)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	// Read output
	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(r)
	output := outputBuf.String()

	// Should have at least one line (empty line)
	assert.Contains(t, output, "\x1b[0m")
}

func TestPrintBannerFromFile_SingleLine(t *testing.T) {
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "single.txt")

	err := os.WriteFile(bannerPath, []byte("Single Line Banner"), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = bs.PrintBannerFromFile(bannerPath)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(r)
	output := outputBuf.String()

	assert.Contains(t, output, "Single Line Banner")
}

func TestPrintBannerFromFile_MultipleLines(t *testing.T) {
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "multi.txt")

	bannerContent := strings.Join([]string{
		"Line 1",
		"Line 2",
		"Line 3",
		"Line 4",
		"Line 5",
		"Line 6",
		"Line 7", // More than 6 lines to test color cycling
	}, "\n")

	err := os.WriteFile(bannerPath, []byte(bannerContent), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = bs.PrintBannerFromFile(bannerPath)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(r)
	output := outputBuf.String()

	// Verify all lines are present
	for i := 1; i <= 7; i++ {
		assert.Contains(t, output, "Line "+string(rune('0'+i)))
	}

	// Verify different colors are used (color cycling)
	assert.Contains(t, output, "\x1b[38;5;165m") // First color
	assert.Contains(t, output, "\x1b[38;5;189m") // Second color
}

func TestPrintBannerFromFile_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "large.txt")

	// Create a large banner with many lines
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "Banner line "+string(rune('0'+i%10)))
	}
	bannerContent := strings.Join(lines, "\n")

	err := os.WriteFile(bannerPath, []byte(bannerContent), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	// Should handle large files without issues
	err = bs.PrintBannerFromFile(bannerPath)
	assert.NoError(t, err)
}

func TestPrintBannerFromFile_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "noperm.txt")

	err := os.WriteFile(bannerPath, []byte("test"), 0644)
	require.NoError(t, err)

	// Remove read permission
	err = os.Chmod(bannerPath, 0000)
	require.NoError(t, err)

	defer os.Chmod(bannerPath, 0644) // Restore for cleanup

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)
	err = bs.PrintBannerFromFile(bannerPath)
	assert.Error(t, err)
}

// Benchmark tests
func BenchmarkPrintBannerFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	bannerPath := filepath.Join(tmpDir, "bench_banner.txt")

	bannerContent := strings.Repeat("Benchmark Banner Line\n", 10)
	err := os.WriteFile(bannerPath, []byte(bannerContent), 0644)
	if err != nil {
		b.Fatal(err)
	}

	// Redirect stdout to discard output during benchmark
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := bs.PrintBannerFromFile(bannerPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test color cycling specifically
func TestPrintBannerFromFile_ColorCycling(t *testing.T) {
	tmpDir := t.TempDir()
	bannerPath := filepath.Join(tmpDir, "colors.txt")

	// Create exactly 12 lines to test color cycling (6 colors * 2)
	lines := make([]string, 12)
	for i := 0; i < 12; i++ {
		lines[i] = "Color test line " + string(rune('A'+i))
	}
	bannerContent := strings.Join(lines, "\n")

	err := os.WriteFile(bannerPath, []byte(bannerContent), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = bs.PrintBannerFromFile(bannerPath)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var outputBuf bytes.Buffer
	outputBuf.ReadFrom(r)
	output := outputBuf.String()

	// Verify that colors cycle (first and seventh line should have same color)
	lines = strings.Split(output, "\n")
	if len(lines) >= 12 {
		// Extract color codes from first and seventh lines
		firstLineColor := extractColorCode(lines[0])
		seventhLineColor := extractColorCode(lines[6])

		assert.Equal(t, firstLineColor, seventhLineColor, "Colors should cycle every 6 lines")
	}
}

// Helper function to extract color code from a line
func extractColorCode(line string) string {
	start := strings.Index(line, "\x1b[38;5;")
	if start == -1 {
		return ""
	}
	end := strings.Index(line[start:], "m")
	if end == -1 {
		return ""
	}
	return line[start : start+end+1]
}
