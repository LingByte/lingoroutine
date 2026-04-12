package utils

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"sync"
)

// sync.Once主要是用来进行延迟初始化全局变量的，而不是使用init方法，因为init方法必然会被执行，
// 类似于数据库连接这种，是按需执行的，但是init方法就启动就会开启数据库连接

// Singleton 泛型单例封装
type Singleton[T any] struct {
	once     sync.Once
	instance T
	initFunc func() T
}

// NewSingleton 创建单例
// initFunc: 初始化函数，返回实例
func NewSingleton[T any](initFunc func() T) *Singleton[T] {
	return &Singleton[T]{
		initFunc: initFunc,
	}
}

// Get 获取单例实例（延迟初始化）
func (s *Singleton[T]) Get() T {
	s.once.Do(func() {
		s.instance = s.initFunc()
	})
	return s.instance
}

// Reset 重置单例（主要用于测试）
func (s *Singleton[T]) Reset() {
	var zero T
	s.instance = zero
	s.once = sync.Once{}
}

// IsInitialized 检查是否已初始化
func (s *Singleton[T]) IsInitialized() bool {
	var zero T
	return s.instance != zero
}
