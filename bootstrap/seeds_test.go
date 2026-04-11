package bootstrap

// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

import (
	"testing"

	"github.com/LingByte/lingoroutine/logger"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	logger.Init(&logger.LogConfig{
		Level:      "info",
		Filename:   "",
		MaxSize:    100,
		MaxAge:     30,
		MaxBackups: 3,
	}, "test")
}

type TestUser struct {
	ID    uint `gorm:"primaryKey"`
	Name  string
	Email string `gorm:"uniqueIndex"`
}

type TestPost struct {
	ID      uint `gorm:"primaryKey"`
	Title   string
	Content string
}

func setupTestDB(t testing.TB) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&TestUser{}, &TestPost{}, &utils.Config{})
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func TestNewSeedService(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)
	assert.NotNil(t, service)
	assert.NotNil(t, service.db)
}

func TestSeedService_SeedAll(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)

	callCount := 0
	seedFunc1 := func(db *gorm.DB) error {
		callCount++
		return nil
	}
	seedFunc2 := func(db *gorm.DB) error {
		callCount++
		return nil
	}

	err := service.SeedAll(seedFunc1, seedFunc2)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestSeedService_SeedAll_Error(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)

	seedFunc1 := func(db *gorm.DB) error {
		return nil
	}
	seedFunc2 := func(db *gorm.DB) error {
		return assert.AnError
	}

	err := service.SeedAll(seedFunc1, seedFunc2)
	assert.Error(t, err)
}

func TestSeedService_SeedAll_Empty(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)

	err := service.SeedAll()
	assert.NoError(t, err)
}

func TestSeedFunc_CustomSeed(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)

	seedFunc := func(db *gorm.DB) error {
		user := TestUser{
			Name:  "Test User",
			Email: "test@example.com",
		}
		return db.Create(&user).Error
	}

	err := service.SeedAll(seedFunc)
	assert.NoError(t, err)

	// Verify user was created
	var count int64
	err = db.Model(&TestUser{}).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	var user TestUser
	err = db.Where("email = ?", "test@example.com").First(&user).Error
	assert.NoError(t, err)
	assert.Equal(t, "Test User", user.Name)
}

func TestSeedFunc_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)

	seedFunc := func(db *gorm.DB) error {
		var count int64
		if err := db.Model(&TestUser{}).Where("email = ?", "test@example.com").Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			user := TestUser{
				Name:  "Test User",
				Email: "test@example.com",
			}
			return db.Create(&user).Error
		}
		return nil
	}

	// Run seed twice
	err := service.SeedAll(seedFunc)
	assert.NoError(t, err)

	err = service.SeedAll(seedFunc)
	assert.NoError(t, err)

	// Verify only one user was created
	var count int64
	err = db.Model(&TestUser{}).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestSeedFunc_MultipleModels(t *testing.T) {
	db := setupTestDB(t)
	service := NewSeedService(db)

	seedUsers := func(db *gorm.DB) error {
		users := []TestUser{
			{Name: "User 1", Email: "user1@example.com"},
			{Name: "User 2", Email: "user2@example.com"},
		}
		for _, user := range users {
			if err := db.Create(&user).Error; err != nil {
				return err
			}
		}
		return nil
	}

	seedPosts := func(db *gorm.DB) error {
		posts := []TestPost{
			{Title: "Post 1", Content: "Content 1"},
			{Title: "Post 2", Content: "Content 2"},
		}
		for _, post := range posts {
			if err := db.Create(&post).Error; err != nil {
				return err
			}
		}
		return nil
	}

	err := service.SeedAll(seedUsers, seedPosts)
	assert.NoError(t, err)

	// Verify users were created
	var userCount int64
	err = db.Model(&TestUser{}).Count(&userCount).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(2), userCount)

	// Verify posts were created
	var postCount int64
	err = db.Model(&TestPost{}).Count(&postCount).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(2), postCount)
}

// Benchmark tests
func BenchmarkSeedService_SeedAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db := setupTestDB(b)
		service := NewSeedService(db)

		seedFunc := func(db *gorm.DB) error {
			user := TestUser{
				Name:  "Test User",
				Email: "test@example.com",
			}
			return db.Create(&user).Error
		}

		b.StartTimer()
		err := service.SeedAll(seedFunc)
		b.StopTimer()

		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSeedFunc_MultipleSeeds(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db := setupTestDB(b)
		service := NewSeedService(db)

		seedFuncs := make([]SeedFunc, 10)
		for j := range seedFuncs {
			seedFuncs[j] = func(db *gorm.DB) error {
				user := TestUser{
					Name:  "Test User",
					Email: "test@example.com",
				}
				return db.Create(&user).Error
			}
		}

		b.StartTimer()
		err := service.SeedAll(seedFuncs...)
		b.StopTimer()

		if err != nil {
			b.Fatal(err)
		}
	}
}
