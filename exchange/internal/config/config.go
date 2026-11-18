// Package config：从环境变量读配置（企业项目常见做法）
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	RedisAddr    string
	RedisPass    string
	JWTSecret    string
	JWTHours     int
	KafkaBrokers []string // 空 = 不启用 Kafka
	KafkaTopic   string
	RankingURL   string // 排行榜微服务，空则不反代
}

func Load() Config {
	hours, _ := strconv.Atoi(getenv("JWT_TTL_HOURS", "24"))
	brokers := getenv("KAFKA_BROKERS", "")
	var brokerList []string
	if brokers != "" {
		for _, b := range strings.Split(brokers, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				brokerList = append(brokerList, b)
			}
		}
	}
	return Config{
		HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:  getenv("DATABASE_URL", "postgres://exchange:exchange@localhost:5432/exchange?sslmode=disable"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:    getenv("REDIS_PASSWORD", ""),
		JWTSecret:    getenv("JWT_SECRET", "exchange-demo-secret-change-me"),
		JWTHours:     hours,
		KafkaBrokers: brokerList,
		KafkaTopic:   getenv("KAFKA_TOPIC", "trades"),
		RankingURL:   getenv("RANKING_URL", "http://127.0.0.1:8081"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
