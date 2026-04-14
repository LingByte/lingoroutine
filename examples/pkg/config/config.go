// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"errors"
	"log"
	"os"
	"strconv"

	"github.com/LingByte/lingoroutine/logger"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/LingByte/lingstorage-sdk-go"
)

// Config 应用主配置（精简版：仅服务、数据库、日志与 LLM）。
type Config struct {
	Server   ServerConfig     `mapstructure:"server"`
	Database DatabaseConfig   `mapstructure:"database"`
	Log      logger.LogConfig `mapstructure:"log"`
	Services ServicesConfig   `mapstructure:"services"`
}

// ServerConfig HTTP 服务相关。
type ServerConfig struct {
	Name      string `env:"SERVER_NAME"`
	Addr      string `env:"ADDR"`
	Mode      string `env:"MODE"`
	APIPrefix string `env:"API_PREFIX"`
}

// DatabaseConfig 数据库连接。
type DatabaseConfig struct {
	Driver string `env:"DB_DRIVER"`
	DSN    string `env:"DSN"`
}

// ServicesConfig 外部服务。
type ServicesConfig struct {
	LLM     LLMConfig     `mapstructure:"llm"`
	Storage StorageConfig `mapstructure:"storage"`
}

// LLMConfig 大模型调用（供后续引擎使用）。
type LLMConfig struct {
	Provider           string `env:"LLM_PROVIDER"` // openai | ollama | lmstudio 等，走 OpenAI 兼容 Chat Completions
	APIKey             string `env:"LLM_API_KEY"`
	BaseURL            string `env:"LLM_BASE_URL"`
	Model              string `env:"LLM_MODEL"`
	MaxSessionMessages int    `env:"LLM_MAX_SESSION_MESSAGES"`
	SummaryModel       string `env:"LLM_SUMMARY_MODEL"`
}

// StorageConfig storage configuration
type StorageConfig struct {
	BaseURL   string `env:"LINGSTORAGE_BASE_URL"`
	APIKey    string `env:"LINGSTORAGE_API_KEY"`
	APISecret string `env:"LINGSTORAGE_API_SECRET"`
	Bucket    string `env:"LINGSTORAGE_BUCKET"`
}

// GlobalConfig 进程级单例，在 Load 成功后可用。
var GlobalConfig *Config

var GlobalStore *lingstorage.Client

// Load 读取环境（含可选 .env / .env.$MODE），填充 GlobalConfig。
func Load() error {
	mode := os.Getenv("MODE")
	if err := utils.LoadEnv(mode); err != nil {
		log.Printf("config: optional env file not loaded: %v (using defaults / OS env)", err)
	}
	GlobalConfig = &Config{
		Server: ServerConfig{
			Name:      getStringOrDefault("SERVER_NAME", "CinyuVerse"),
			Addr:      getStringOrDefault("ADDR", ":8080"),
			Mode:      getStringOrDefault("MODE", "development"),
			APIPrefix: getStringOrDefault("API_PREFIX", "/api"),
		},
		Database: DatabaseConfig{
			Driver: getStringOrDefault("DB_DRIVER", "sqlite"),
			DSN:    getStringOrDefault("DSN", "./data/cinyuverse.db"),
		},
		Log: logger.LogConfig{
			Level:      getStringOrDefault("LOG_LEVEL", "info"),
			Filename:   getStringOrDefault("LOG_FILENAME", "./logs/cinyuverse.log"),
			MaxSize:    getIntOrDefault("LOG_MAX_SIZE", 32),
			MaxAge:     getIntOrDefault("LOG_MAX_AGE", 14),
			MaxBackups: getIntOrDefault("LOG_MAX_BACKUPS", 5),
			Daily:      getBoolOrDefault("LOG_DAILY", false),
		},
		Services: ServicesConfig{
			LLM: LLMConfig{
				Provider:           getStringOrDefault("LLM_PROVIDER", "openai"),
				APIKey:             getStringOrDefault("LLM_API_KEY", ""),
				BaseURL:            getStringOrDefault("LLM_BASE_URL", "https://api.openai.com/v1"),
				Model:              getStringOrDefault("LLM_MODEL", "gpt-4o-mini"),
				MaxSessionMessages: getIntOrDefault("LLM_MAX_SESSION_MESSAGES", 10),
				SummaryModel:       getStringOrDefault("LLM_SUMMARY_MODEL", ""),
			},
			Storage: StorageConfig{
				BaseURL:   getStringOrDefault("LINGSTORAGE_BASE_URL", "https://api.lingstorage.com"),
				APIKey:    getStringOrDefault("LINGSTORAGE_API_KEY", ""),
				APISecret: getStringOrDefault("LINGSTORAGE_API_SECRET", ""),
				Bucket:    getStringOrDefault("LINGSTORAGE_BUCKET", "default"),
			},
		},
	}
	GlobalStore = lingstorage.NewClient(&lingstorage.Config{
		BaseURL:   GlobalConfig.Services.Storage.BaseURL,
		APIKey:    GlobalConfig.Services.Storage.APIKey,
		APISecret: GlobalConfig.Services.Storage.APISecret,
	})
	return nil
}

// Validate 校验关键字段。
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.Database.DSN == "" {
		return errors.New("database DSN is required")
	}
	if c.Server.Addr == "" {
		return errors.New("server address is required")
	}
	return nil
}

func getStringOrDefault(key, defaultValue string) string {
	if v := utils.GetEnv(key); v != "" {
		return v
	}
	return defaultValue
}

func getBoolOrDefault(key string, defaultValue bool) bool {
	if utils.GetEnv(key) == "" {
		return defaultValue
	}
	return utils.GetBoolEnv(key)
}

func getIntOrDefault(key string, defaultValue int) int {
	if utils.GetEnv(key) == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(utils.GetEnv(key))
	if err != nil {
		return defaultValue
	}
	return v
}

// LogMode 将 Server.Mode 映射为 logger.Init 的 mode 参数。
func (c *Config) LogMode() string {
	if c.Server.Mode == "production" || c.Server.Mode == "prod" {
		return "prod"
	}
	return "dev"
}
