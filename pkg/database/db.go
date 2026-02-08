// Package database provides database connectivity and migration management
package database

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/pkg/config"
)

// VerboseLogger implements the migrate.Logger interface for verbose logging
type VerboseLogger struct {
	verbose bool
}

func (l *VerboseLogger) Printf(format string, v ...interface{}) {
	log.Printf("[MIGRATE] "+format, v...)
}

func (l *VerboseLogger) Verbose() bool {
	return l.verbose
}

// DB wraps the GORM database connection with additional functionality
type DB struct {
	*gorm.DB
	config *config.DatabaseConfig
}

// NewDatabase creates a new database connection using the provided configuration
func NewDatabase(cfg *config.DatabaseConfig) (*DB, error) {
	dbUrl := cfg.ConnectionString()

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	}

	db, err := gorm.Open(postgres.Open(dbUrl), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return &DB{
		DB:     db,
		config: cfg,
	}, nil
}

// Health checks the database connection health
func (db *DB) Health() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Ping()
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Close()
}

// Migrate runs database migrations
func (db *DB) Migrate(migrationsPath string) error {
	migrationURL := db.config.MigrationURL()

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		migrationURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Enable verbose logging
	m.Log = &VerboseLogger{verbose: true}

	// Log current version before migration
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Printf("[MIGRATE] Failed to get current version: %v\n", err)
	} else {
		log.Printf("[MIGRATE] Current migration version: %d, dirty: %v\n", version, dirty)
	}

	log.Printf("[MIGRATE] Starting migrations from path: %s\n", migrationsPath)

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Printf("[MIGRATE] No migrations to apply\n")
	} else {
		log.Printf("[MIGRATE] Migrations completed successfully\n")
	}

	return nil
}

// MigrateDown rolls back database migrations
func (db *DB) MigrateDown(migrationsPath string, steps int) error {
	migrationURL := db.config.MigrationURL()

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		migrationURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(fn func(*gorm.DB) error) error {
	return db.DB.Transaction(fn)
}
