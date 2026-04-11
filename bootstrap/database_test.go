package bootstrap

// Copyright (c) 2026 LingByte. All rights reserved.
// SPDX-License-Identifier: AGPL-3.0

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LingByte/lingoroutine/logger"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type TestModel struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func init() {
	logger.Init(&logger.LogConfig{
		Level:    "info",
		Filename: "",
		MaxSize:  100,
		MaxAge:   30,
		MaxBackups: 3,
	}, "test")
}

func TestNewBootstrap(t *testing.T) {
	var buf bytes.Buffer
	
	// Test with nil options
	bs := NewBootstrap(&buf, nil)
	assert.NotNil(t, bs)
	assert.NotNil(t, bs.options)
	assert.False(t, bs.options.AutoMigrate)
	assert.False(t, bs.options.SeedNonProd)
	assert.Equal(t, "cmd/bootstrap/migrations", bs.options.MigrationsDir)
	
	// Test with custom options
	opts := &Options{
		AutoMigrate:   true,
		SeedNonProd:   true,
		MigrationsDir: "./migrations",
		DBDriver:      "sqlite",
		DSN:           ":memory:",
	}
	bs = NewBootstrap(&buf, opts)
	assert.NotNil(t, bs)
	assert.True(t, bs.options.AutoMigrate)
	assert.True(t, bs.options.SeedNonProd)
	assert.Equal(t, "./migrations", bs.options.MigrationsDir)
}

func TestSetupDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	var buf bytes.Buffer
	opts := &Options{
		DBDriver:    "sqlite",
		DSN:         dbPath,
		AutoMigrate: true,
		SeedNonProd: false,
		Models:      []any{&TestModel{}},
	}

	bs := NewBootstrap(&buf, opts)
	db, err := bs.SetupDatabase()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Verify database connection
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())

	// Verify table was created
	assert.True(t, db.Migrator().HasTable("test_models"))

	// Clean up
	sqlDB.Close()
}

func TestSetupDatabase_WithInitSQL(t *testing.T) {
	// Create temporary database and SQL file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	sqlPath := filepath.Join(tmpDir, "init.sql")

	// Create test SQL file
	sqlContent := `
-- Test comment
CREATE TABLE IF NOT EXISTS test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT OR IGNORE INTO test_table (id, name) VALUES (1, 'test');
`
	err := os.WriteFile(sqlPath, []byte(sqlContent), 0644)
	require.NoError(t, err)

	var buf bytes.Buffer
	opts := &Options{
		DBDriver:    "sqlite",
		DSN:         dbPath,
		InitSQLPath: sqlPath,
		AutoMigrate: false,
		SeedNonProd: false,
	}

	bs := NewBootstrap(&buf, opts)
	db, err := bs.SetupDatabase()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Verify SQL was executed
	var count int64
	err = db.Table("test_table").Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Clean up
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestSetupDatabase_NilOptions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	var buf bytes.Buffer
	bs := NewBootstrap(&buf, nil)
	bs.options.DBDriver = "sqlite"
	bs.options.DSN = dbPath

	db, err := bs.SetupDatabase()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Clean up
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestRunInitSQL(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create temporary SQL file
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "test.sql")

	sqlContent := `
-- This is a comment
CREATE TABLE test_users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

-- Another comment
INSERT INTO test_users (name, email) VALUES ('John', 'john@example.com');
INSERT INTO test_users (name, email) VALUES ('Jane', 'jane@example.com');

# Hash comment
CREATE TABLE test_posts (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL
);
`
	err = os.WriteFile(sqlPath, []byte(sqlContent), 0644)
	require.NoError(t, err)

	// Run SQL
	err = RunInitSQL(db, sqlPath)
	require.NoError(t, err)

	// Verify tables were created and data inserted
	var userCount int64
	err = db.Table("test_users").Count(&userCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), userCount)

	var postCount int64
	err = db.Table("test_posts").Count(&postCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), postCount)
}

func TestRunInitSQL_FileNotFound(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunInitSQL(db, "/nonexistent/file.sql")
	assert.Error(t, err)
}

func TestRunInitSQL_EmptyFile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "empty.sql")
	err = os.WriteFile(sqlPath, []byte(""), 0644)
	require.NoError(t, err)

	err = RunInitSQL(db, sqlPath)
	assert.NoError(t, err)
}

func TestRunInitSQL_OnlyComments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "comments.sql")
	sqlContent := `
-- This is a comment
# This is also a comment

-- Another comment
`
	err = os.WriteFile(sqlPath, []byte(sqlContent), 0644)
	require.NoError(t, err)

	err = RunInitSQL(db, sqlPath)
	assert.NoError(t, err)
}

func TestRunInitSQL_StatementWithoutSemicolon(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "no_semicolon.sql")
	sqlContent := `CREATE TABLE test_table (id INTEGER PRIMARY KEY)`
	err = os.WriteFile(sqlPath, []byte(sqlContent), 0644)
	require.NoError(t, err)

	err = RunInitSQL(db, sqlPath)
	assert.NoError(t, err)

	// Verify table was created
	var count int64
	err = db.Table("test_table").Count(&count).Error
	require.NoError(t, err)
}

func TestRunInitSQL_InvalidSQL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "invalid.sql")
	sqlContent := `INVALID SQL STATEMENT;`
	err = os.WriteFile(sqlPath, []byte(sqlContent), 0644)
	require.NoError(t, err)

	err = RunInitSQL(db, sqlPath)
	assert.Error(t, err)
}

func TestRunMigrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db, []any{&TestModel{}, &utils.Config{}})
	assert.NoError(t, err)

	// Verify tables were created
	assert.True(t, db.Migrator().HasTable("test_models"))
	assert.True(t, db.Migrator().HasTable("configs"))
}

func TestRunMigrations_NilDB(t *testing.T) {
	err := RunMigrations(nil, []any{&TestModel{}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db is nil")
}

func TestRunMigrations_EmptyModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db, []any{})
	assert.NoError(t, err)
}

func TestSetupDatabase_ProductionEnvironment(t *testing.T) {
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Set production environment
	os.Setenv("APP_ENV", "production")

	var buf bytes.Buffer
	opts := &Options{
		DBDriver:    "sqlite",
		DSN:         dbPath,
		AutoMigrate: true,
		SeedNonProd: true, // Should be ignored in production
		Models:      []any{&TestModel{}},
		SeedFuncs:   []SeedFunc{func(db *gorm.DB) error { return nil }},
	}

	bs := NewBootstrap(&buf, opts)
	db, err := bs.SetupDatabase()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Clean up
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestSetupDatabase_WithSeedFunctions(t *testing.T) {
	originalEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", originalEnv)

	os.Setenv("APP_ENV", "test")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	seedCalled := false
	seedFunc := func(db *gorm.DB) error {
		seedCalled = true
		return nil
	}

	var buf bytes.Buffer
	opts := &Options{
		DBDriver:    "sqlite",
		DSN:         dbPath,
		AutoMigrate: true,
		SeedNonProd: true,
		Models:      []any{&TestModel{}},
		SeedFuncs:   []SeedFunc{seedFunc},
	}

	bs := NewBootstrap(&buf, opts)
	db, err := bs.SetupDatabase()
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.True(t, seedCalled)

	// Clean up
	sqlDB, _ := db.DB()
	sqlDB.Close()
}

func TestSetupDatabase_DatabaseConnectionError(t *testing.T) {
	var buf bytes.Buffer
	opts := &Options{
		DBDriver:    "mysql",
		DSN:         "invalid:invalid@tcp(nonexistent:3306)/nonexistent?charset=utf8mb4&parseTime=True&loc=Local",
		AutoMigrate: true,
		SeedNonProd: false,
	}

	bs := NewBootstrap(&buf, opts)
	db, err := bs.SetupDatabase()
	assert.Error(t, err)
	assert.Nil(t, db)
}

// Benchmark tests
func BenchmarkSetupDatabase(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		dbPath := filepath.Join(tmpDir, "bench.db")

		var buf bytes.Buffer
		opts := &Options{
			DBDriver:    "sqlite",
			DSN:         dbPath,
			AutoMigrate: true,
			SeedNonProd: false,
			Models:      []any{&TestModel{}},
		}

		bs := NewBootstrap(&buf, opts)
		db, err := bs.SetupDatabase()
		if err != nil {
			b.Fatal(err)
		}

		sqlDB, _ := db.DB()
		sqlDB.Close()
	}
}

func BenchmarkRunInitSQL(b *testing.B) {
	// Create test SQL content
	sqlContent := strings.Repeat("INSERT INTO test_table (name) VALUES ('test');\n", 100)

	for i := 0; i < b.N; i++ {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			b.Fatal(err)
		}

		// Create table first
		err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)").Error
		if err != nil {
			b.Fatal(err)
		}

		tmpDir := b.TempDir()
		sqlPath := filepath.Join(tmpDir, "bench.sql")
		err = os.WriteFile(sqlPath, []byte(sqlContent), 0644)
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
		err = RunInitSQL(db, sqlPath)
		b.StopTimer()

		if err != nil {
			b.Fatal(err)
		}
	}
}
