package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env      string 
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	App      AppConfig
}

type ServerConfig struct {
	Port    string
	GinMode string
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type AppConfig struct {
	AllowedOrigins      []string
	ScoreUpdateInterval time.Duration
	MaxSearchResults    int
}

var AppCfg *Config

func LoadConfig() *Config {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}


	cfg := &Config{
		Env:  getEnv("APP_ENV", "development"),
		Server: ServerConfig{
			Port:    getEnv("PORT", "8080"),
			GinMode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			URL: getEnv("DB_URL", "localhost"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
		App: AppConfig{
			AllowedOrigins: []string{
				"http://localhost:8081",
				"http://localhost:19006",
				"https://yourdomain.vercel.app",
			},
			ScoreUpdateInterval: 3 * time.Second,
			MaxSearchResults:    100,
		},
	}

	AppCfg = cfg
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *DatabaseConfig) DSN() string {
	return c.URL
}

func (c *RedisConfig) Address() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

func IsProduction() bool {
	return AppCfg != nil && AppCfg.Env == "production"
}

func IsDevelopment() bool {
	return AppCfg != nil && AppCfg.Env == "development"
}

func IsStaging() bool {
	return AppCfg != nil && AppCfg.Env == "staging"
}