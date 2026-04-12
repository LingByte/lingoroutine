.PHONY: help build test clean install lint fmt vet check

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')
GO_VERSION ?= $(shell go version | awk '{print $$3}')

# 构建标志
LDFLAGS := -ldflags "-X 'github.com/LingByte/lingoroutine/version.Version=$(VERSION)' \
                      -X 'github.com/LingByte/lingoroutine/version.GitCommit=$(GIT_COMMIT)' \
                      -X 'github.com/LingByte/lingoroutine/version.BuildTime=$(BUILD_TIME)' \
                      -X 'github.com/LingByte/lingoroutine/version.GoVersion=$(GO_VERSION)'"

help: ## 显示帮助信息
	@echo "Usage: make [target]"
	@echo ""
	@echo "可用目标:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## 构建项目
	@echo "Building version $(VERSION)..."
	go build $(LDFLAGS) -v ./...

test: ## 运行测试
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-short: ## 运行快速测试
	go test -v -short ./...

benchmark: ## 运行性能测试
	go test -bench=. -benchmem ./...

lint: ## 运行代码检查
	golangci-lint run ./...

fmt: ## 格式化代码
	go fmt ./...
	goimports -w .

vet: ## 运行go vet
	go vet ./...

check: fmt vet lint ## 运行所有检查

install: ## 安装依赖
	go mod download
	go mod tidy

clean: ## 清理构建文件
	rm -f coverage.out coverage.html
	go clean ./...

version: ## 显示版本信息
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(GO_VERSION)"

tag: ## 创建Git标签
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is required. Usage: make tag VERSION=v1.0.0"; \
		exit 1; \
	fi
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
