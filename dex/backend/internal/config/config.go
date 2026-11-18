package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr      string
	JWTSecret     string
	Maintenance   bool
	Version       string
	AllowOrigins  string
}

func Load() Config {
	return Config{
		HTTPAddr:     getenv("HTTP_ADDR", ":8088"),
		JWTSecret:    getenv("JWT_SECRET", "tf-dex-2026-demo-secret"),
		Maintenance:  getenv("MAINTENANCE", "false") == "true",
		Version:      getenv("APP_VERSION", "2026.1.0"),
		AllowOrigins: getenv("ALLOW_ORIGINS", "http://localhost:5173"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func EnvInt(k string, def int) int {
	n, err := strconv.Atoi(getenv(k, ""))
	if err != nil {
		return def
	}
	return n
}
