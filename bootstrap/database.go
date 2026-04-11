package bootstrap

// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/LingByte/lingoroutine/logger"
	"github.com/LingByte/lingoroutine/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Options controls database initialization behavior
type Options struct {
	// InitSQLPath points to a .sql script file (optional); skip if empty
	InitSQLPath string
	// AutoMigrate whether to execute entity migration (default true)
	AutoMigrate bool
	// SeedNonProd whether to write default configuration in non-production environments (default true)
	SeedNonProd bool
	// MigrationsDir path to migration SQL scripts directory
	MigrationsDir string
	// DBDriver database driver
	DBDriver string
	// DSN database connection string
	DSN string
	// Models list of models to migrate
	Models []any
	// SeedFuncs list of seed functions to execute after migration
	SeedFuncs []SeedFunc
}

// Bootstrap represents the bootstrap service
type Bootstrap struct {
	options   *Options
	logWriter io.Writer
}

// NewBootstrap creates a new Bootstrap instance
func NewBootstrap(logWriter io.Writer, opts *Options) *Bootstrap {
	if opts == nil {
		opts = &Options{AutoMigrate: false, SeedNonProd: false}
	}
	if opts.MigrationsDir == "" {
		opts.MigrationsDir = "cmd/bootstrap/migrations"
	}
	return &Bootstrap{
		options:   opts,
		logWriter: logWriter,
	}
}

// SetupDatabase unified entry: connect database -> run initialization SQL -> migrate entities -> (non-production) write default configuration
func (b *Bootstrap) SetupDatabase() (*gorm.DB, error) {
	// 1) Connect to database
	db, err := b.initDBConn()
	if err != nil {
		logger.Lg.Error("init database failed", zap.Error(err))
		return nil, err
	}

	// 2) Optional: execute initialization SQL
	if b.options.InitSQLPath != "" {
		if err := RunInitSQL(db, b.options.InitSQLPath); err != nil {
			logger.Lg.Error("run init sql failed", zap.String("path", b.options.InitSQLPath), zap.Error(err))
			return nil, err
		}
	}

	// 3) Migrate entities
	if b.options.AutoMigrate {
		if err := runMigrationScripts(db, b.options.MigrationsDir); err != nil {
			logger.Lg.Warn("run migration scripts failed", zap.String("dir", b.options.MigrationsDir), zap.Error(err))
		}

		if err := RunMigrations(db, b.options.Models); err != nil {
			logger.Lg.Error("migration failed", zap.Error(err))
			return nil, err
		}
		logger.Lg.Info("migration success", zap.String("driver", b.options.DBDriver))
	}

	// 4) Seed data if seed functions are provided
	if len(b.options.SeedFuncs) > 0 {
		if b.options.SeedNonProd && utils.GetEnv("APP_ENV") != "production" && utils.GetEnv("APP_ENV") != "development" {
			service := NewSeedService(db)
			if err := service.SeedAll(b.options.SeedFuncs...); err != nil {
				logger.Lg.Error("seed failed", zap.Error(err))
				return nil, err
			}
		}
	}

	logger.Lg.Info("system bootstrap - database is initialization complete")
	return db, nil
}

// initDBConn creates *gorm.DB based on options
func (b *Bootstrap) initDBConn() (*gorm.DB, error) {
	dbDriver := b.options.DBDriver
	dsn := b.options.DSN
	return utils.InitDatabase(b.logWriter, dbDriver, dsn)
}

// RunInitSQL executes SQL statements from a local .sql file segment by segment (split by semicolon ;), idempotent scripts should use IF NOT EXISTS in SQL for protection
func RunInitSQL(db *gorm.DB, sqlFilePath string) error {
	f, err := os.Open(sqlFilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	var (
		sb      strings.Builder
		scanner = bufio.NewScanner(f)
	)
	// Relax token limit (long lines)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		// Ignore comment lines (starting with --) and empty lines
		if trim == "" || strings.HasPrefix(trim, "--") || strings.HasPrefix(trim, "#") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
		// Use ; as statement terminator (simple splitting, suitable for most scenarios)
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(sb.String())
			sb.Reset()
			if stmt != "" {
				if err := db.Exec(stmt).Error; err != nil {
					return err
				}
			}
		}
	}
	// Handle remaining content at end of file without semicolon
	rest := strings.TrimSpace(sb.String())
	if rest != "" {
		if err := db.Exec(rest).Error; err != nil {
			return err
		}
	}
	return scanner.Err()
}

// runMigrationScripts executes all .sql files in the migrations directory
func runMigrationScripts(db *gorm.DB, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		filePath := migrationsDir + "/" + entry.Name()
		if err := RunInitSQL(db, filePath); err != nil {
			return err
		}
	}
	return nil
}

// RunMigrations executes entity migration
func RunMigrations(db *gorm.DB, models []any) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if len(models) == 0 {
		return nil
	}
	return utils.MakeMigrates(db, models)
}
