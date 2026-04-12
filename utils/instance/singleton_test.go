package utils

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"sync"
	"testing"
)

type Config struct {
	Name  string
	Value int
}

func TestSingleton(t *testing.T) {
	callCount := 0
	singleton := NewSingleton(func() *Config {
		callCount++
		return &Config{Name: "test", Value: 42}
	})

	// 第一次调用应该初始化
	instance1 := singleton.Get()
	if instance1.Name != "test" || instance1.Value != 42 {
		t.Error("instance not initialized correctly")
	}
	if callCount != 1 {
		t.Errorf("initFunc should be called once, got %d", callCount)
	}

	// 第二次调用应该返回同一个实例
	instance2 := singleton.Get()
	if instance1 != instance2 {
		t.Error("should return the same instance")
	}
	if callCount != 1 {
		t.Errorf("initFunc should still be called once, got %d", callCount)
	}
}

func TestSingletonConcurrency(t *testing.T) {
	callCount := 0
	singleton := NewSingleton(func() *Config {
		callCount++
		return &Config{Name: "concurrent", Value: 100}
	})

	var wg sync.WaitGroup
	instances := make([]*Config, 100)

	// 并发调用
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			instances[idx] = singleton.Get()
		}(i)
	}
	wg.Wait()

	// 所有实例应该是同一个
	first := instances[0]
	for _, inst := range instances {
		if inst != first {
			t.Error("all instances should be the same")
		}
	}

	// 初始化函数应该只调用一次
	if callCount != 1 {
		t.Errorf("initFunc should be called once in concurrent scenario, got %d", callCount)
	}
}

func TestSingletonReset(t *testing.T) {
	callCount := 0
	singleton := NewSingleton(func() *Config {
		callCount++
		return &Config{Name: "reset", Value: 1}
	})

	// 第一次初始化
	instance1 := singleton.Get()
	if callCount != 1 {
		t.Errorf("initFunc should be called once, got %d", callCount)
	}

	// 重置
	singleton.Reset()

	// 再次获取应该重新初始化
	instance2 := singleton.Get()
	if callCount != 2 {
		t.Errorf("initFunc should be called twice after reset, got %d", callCount)
	}
	if instance1 == instance2 {
		t.Error("instances should be different after reset")
	}
}

func TestSingletonIsInitialized(t *testing.T) {
	singleton := NewSingleton(func() *Config {
		return &Config{Name: "check", Value: 1}
	})

	// 未初始化时
	if singleton.IsInitialized() {
		t.Error("should not be initialized before Get()")
	}

	// 初始化后
	singleton.Get()
	if !singleton.IsInitialized() {
		t.Error("should be initialized after Get()")
	}
}
