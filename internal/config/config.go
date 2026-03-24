package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	CSVPath            string
	ExternalAPIURL     string
	APITimeout         time.Duration
	CacheRefreshPeriod time.Duration
	RedisAddr          string
	RedisPass          string
}

func LoadConfig() *Config {
	// _ = godotenv.Load()
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	return &Config{
		CSVPath:            getEnv("CSV_PATH", "HDBCarparkInformation.csv"),
		ExternalAPIURL:     getEnv("LIVE_CARPARK_API_URL", "https://api.data.gov.sg/v1/transport/carpark-availability"),
		APITimeout:         time.Duration(getEnvAsInt("API_TIMEOUT_SEC", 10)) * time.Second,
		CacheRefreshPeriod: time.Duration(getEnvAsInt("CACHE_REFRESH_PERIOD_SEC", 60)) * time.Second,
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:          getEnv("REDIS_PASS", ""),
	}
}

// Helper functions to read environment variables with defaults
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultVal
}
