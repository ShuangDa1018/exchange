// Package config：从环境变量读配置（企业项目常见做法）
package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	RedisAddr   string
	RedisPass   string
}

func Load() Config {
	return Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://exchange:exchange@localhost:5432/exchange?sslmode=disable"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getenv("REDIS_PASSWORD", ""),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
