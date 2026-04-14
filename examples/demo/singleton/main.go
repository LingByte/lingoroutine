package main

import (
	"fmt"
	"sync"

	"github.com/LingByte/lingoroutine/utils/instance"
)

// DatabaseConfig 数据库配置单例
type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

var dbConfigSingleton = instance.NewSingleton(func() *DatabaseConfig {
	fmt.Println("初始化数据库配置...")
	return &DatabaseConfig{
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "password",
		Database: "mydb",
	}
})

// CacheConfig 缓存配置单例
type CacheConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

var cacheConfigSingleton = instance.NewSingleton(func() *CacheConfig {
	fmt.Println("初始化缓存配置...")
	return &CacheConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		DB:       0,
	}
})

func main() {
	fmt.Println("=== 单例模式示例 ===")
	fmt.Println()

	// 获取数据库配置
	dbConfig := dbConfigSingleton.Get()
	fmt.Printf("数据库配置: %+v\n", dbConfig)
	fmt.Println()

	// 再次获取（不会重新初始化）
	dbConfig2 := dbConfigSingleton.Get()
	fmt.Printf("再次获取数据库配置: %+v\n", dbConfig2)
	fmt.Printf("是否为同一实例: %v\n", dbConfig == dbConfig2)
	fmt.Println()

	// 并发测试
	fmt.Println("=== 并发测试 ===")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			config := cacheConfigSingleton.Get()
			fmt.Printf("Goroutine %d 获取缓存配置: %+v\n", id, config)
		}(i)
	}
	wg.Wait()

	fmt.Println()
	fmt.Println("=== 检查初始化状态 ===")
	fmt.Printf("数据库配置已初始化: %v\n", dbConfigSingleton.IsInitialized())
	fmt.Printf("缓存配置已初始化: %v\n", cacheConfigSingleton.IsInitialized())
}
