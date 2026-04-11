//go:build pg

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

package utils

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func createDatabaseInstance(cfg *gorm.Config, driver, dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), cfg)
}
