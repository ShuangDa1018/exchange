// 入口文件：组装各模块并启动 HTTP 服务（Gin）
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

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/api"
	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/config"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/store"
	"github.com/gin-gonic/gin"
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

	// 4) 撮合引擎 + Gin API
	eng := engine.New("BTC-USDT", "BTC", "USDT", ledger, db)
	apiServer := api.New(ledger, eng, rdb)
	r := apiServer.Engine()

	// 5) 非 /api 路径走静态前端
	web, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(web))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	log.Printf("交易所 Demo 已启动 (Gin): http://localhost%s", cfg.HTTPAddr)
	log.Printf("Postgres + Redis 已连接 | 交易对 BTC-USDT | 用户 alice/bob")
	log.Printf("  GET  /api/book | /api/trades | /api/balances | /api/ledger")
	log.Printf("  POST /api/orders | DELETE /api/orders/{id}")

	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
