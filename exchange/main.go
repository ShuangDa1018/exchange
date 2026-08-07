// 入口文件：组装各模块并启动 HTTP 服务
// 依赖：docker compose up -d  （Postgres + Redis）
// 运行：cd exchange && go run .
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/api"
	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/config"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/store"
)

//go:embed static/*
var staticFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	cfg := config.Load()
	ctx := context.Background()

	// 1) [DB] 连接 Postgres + [DB:MIGRATE] 跑 migrations
	db, err := store.Open(ctx, cfg.DatabaseURL, migrationsFS)
	if err != nil {
		log.Fatalf("Postgres 不可用（先执行 docker compose up -d）: %v", err)
	}
	defer db.Close()

	// 2) [Redis] 连接 Redis（余额缓存 + API 限流）
	rdb, err := cache.Open(cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Fatalf("Redis 不可用（先执行 docker compose up -d）: %v", err)
	}
	defer rdb.Close()

	// 3) 账本（内部会 [DB:READ] 加载余额、[Redis:WRITE] 预热）
	ledger := account.NewLedger(db, rdb)
	has, err := db.HasAnyBalance(ctx) // [DB:READ]
	if err != nil {
		log.Fatal(err)
	}
	if !has {
		ledger.EnsureUser("alice", map[string]float64{"USDT": 10000, "BTC": 1}) // [DB:WRITE]
		ledger.EnsureUser("bob", map[string]float64{"USDT": 10000, "BTC": 1})   // [DB:WRITE]
		log.Printf("已初始化演示用户 alice / bob")
	}

	// 4) 撮合引擎（内部会 [DB:READ] 恢复挂单）
	eng := engine.New("BTC-USDT", "BTC", "USDT", ledger, db)
	apiServer := api.New(ledger, eng, rdb)

	web, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(web))

	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("交易所 Demo 已启动: http://localhost%s", cfg.HTTPAddr)
	log.Printf("Postgres + Redis 已连接 | 交易对 BTC-USDT | 用户 alice/bob")
	log.Printf("  GET  /api/book | /api/trades | /api/balances | /api/ledger")
	log.Printf("  POST /api/orders | DELETE /api/orders/{id}")

	if err := http.ListenAndServe(cfg.HTTPAddr, withCORS(withRateLimit(rdb, root))); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRateLimit [Redis:WRITE] 每个 IP 每分钟最多 120 次请求
func withRateLimit(rdb *cache.Redis, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()
		ok, err := rdb.Allow(ctx, ip, 120, time.Minute) // [Redis:WRITE]
		if err == nil && !ok {
			http.Error(w, `{"error":"请求过于频繁"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
