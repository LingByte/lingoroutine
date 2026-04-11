package logger

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	Daily      bool   `mapstructure:"daily"`
}

var (
	Lg *zap.Logger
)

// Init 初始化logger
func Init(cfg *LogConfig, mode string) (err error) {
	writeSyncer := getLogWriter(cfg.Filename, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge, cfg.Daily)
	encoder := getEncoder()
	var l = new(zapcore.Level)
	err = l.UnmarshalText([]byte(cfg.Level))
	if err != nil {
		return
	}
	var core zapcore.Core
	if mode == "dev" || mode == "development" {
		consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		consoleEncoderConfig.TimeKey = "time"
		consoleEncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		consoleEncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString("\x1b[90m" + t.Format("2006-01-02 15:04:05.000") + "\x1b[0m")
		}
		consoleEncoderConfig.EncodeLevel = func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
			var levelColor = map[zapcore.Level]string{
				zapcore.DebugLevel:  "\x1b[35m", // 紫色
				zapcore.InfoLevel:   "\x1b[36m", // 青色
				zapcore.WarnLevel:   "\x1b[33m", // 黄色
				zapcore.ErrorLevel:  "\x1b[31m", // 红色
				zapcore.DPanicLevel: "\x1b[31m", // 红色
				zapcore.PanicLevel:  "\x1b[31m", // 红色
				zapcore.FatalLevel:  "\x1b[31m", // 红色
			}
			color, ok := levelColor[l]
			if !ok {
				color = "\x1b[0m"
			}
			enc.AppendString(color + "[" + l.CapitalString() + "]\x1b[0m")
		}
		consoleEncoderConfig.EncodeCaller = func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString("\x1b[90m" + caller.TrimmedPath() + "\x1b[0m")
		}
		consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
		highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.ErrorLevel
		})
		lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl < zapcore.ErrorLevel
		})
		core = zapcore.NewTee(
			zapcore.NewCore(encoder, writeSyncer, l),
			zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), lowPriority),
			zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), highPriority),
		)
	} else {
		core = zapcore.NewCore(encoder, writeSyncer, l)
	}
	Lg = zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(Lg)
	Info("init logger success")
	return
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(filename string, maxSize, maxBackup, maxAge int, daily bool) zapcore.WriteSyncer {
	if daily {
		ext := filepath.Ext(filename)
		base := filename[:len(filename)-len(ext)]
		dateStr := time.Now().Format("2006-01-02")
		filename = base + "-" + dateStr + ext
	}
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackup,
		MaxAge:     maxAge,
		LocalTime:  true,
	}
	return zapcore.AddSync(lumberJackLogger)
}

// Info 通用 info 日志方法
func Info(format string, a ...any) {
	Lg.Info(fmt.Sprintf(format, a))
}

// Warn 通用 warn 日志方法
func Warn(format string, a ...any) {
	Lg.Warn(fmt.Sprintf(format, a))
}

// Error 通用 error 日志方法
func Error(format string, a ...any) {
	Lg.Error(fmt.Sprintf(format, a))
}

// Debug 通用 debug 日志方法
func Debug(format string, a ...any) {
	Lg.Debug(fmt.Sprintf(format, a))
}

// Fatal 通用 fatal 日志方法
func Fatal(format string, a ...any) {
	Lg.Fatal(fmt.Sprintf(format, a))
}

// Panic 通用 panic 日志方法
func Panic(format string, a ...any) {
	Lg.Panic(fmt.Sprintf(format, a))
}

// Sync 刷新缓冲区
func Sync() {
	_ = Lg.Sync()
}
