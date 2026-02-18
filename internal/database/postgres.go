package database

import (
	"fmt"
	"log"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/config"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectPostgres initializes PostgreSQL connection
func ConnectPostgres(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := cfg.DSN()

	// Configure GORM logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	log.Println("✅ PostgreSQL connected successfully")

	DB = db
	return db, nil
}

// AutoMigrate runs database migrations
func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	err := db.AutoMigrate(
		&models.User{},
		&models.ScoreUpdate{},
	)

	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Create additional indexes
	if err := createIndexes(db); err != nil {
		return err
	}

	log.Println("✅ Database migrations completed")
	return nil
}

// createIndexes creates additional indexes for better performance
func createIndexes(db *gorm.DB) error {
	// Create trigram extension for fuzzy search (if not exists)
	db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm")

	// Create GIN index for fast username search
	db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_username_trgm 
		ON users USING gin(username gin_trgm_ops)
	`)

	// Create composite index for rating + username queries
	db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_rating_username 
		ON users(rating DESC, username)
	`)

	log.Println("✅ Additional indexes created")
	return nil
}

// CloseDB closes the database connection
func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
