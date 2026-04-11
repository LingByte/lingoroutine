package bootstrap

// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

import (
	"gorm.io/gorm"
)

// SeedService handles database seeding operations
type SeedService struct {
	db *gorm.DB
}

// NewSeedService creates a new SeedService instance
func NewSeedService(db *gorm.DB) *SeedService {
	return &SeedService{db: db}
}

// SeedFunc represents a seeding function
type SeedFunc func(db *gorm.DB) error

// SeedAll executes all registered seed functions
func (s *SeedService) SeedAll(seedFuncs ...SeedFunc) error {
	for _, fn := range seedFuncs {
		if err := fn(s.db); err != nil {
			return err
		}
	}
	return nil
}
