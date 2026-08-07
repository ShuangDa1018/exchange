// 入口文件：组装各模块并启动 HTTP 服务（Gin）
// 依赖：docker compose up -d
// 运行：cd exchange && go run .
package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/api"
	"github.com/demo/exchange/internal/auth"
	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/config"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/mq"
	"github.com/demo/exchange/internal/store"
	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	ctx := context.Background()

	db, err := store.Open(ctx, cfg.DatabaseURL, migrationsFS)
	if err != nil {
		log.Error("postgres unavailable", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := cache.Open(cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Error("redis unavailable", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	ledger := account.NewLedger(db, rdb)
	has, err := db.HasAnyBalance(ctx)
	if err != nil {
		log.Error("check balances", "err", err)
		os.Exit(1)
	}
	if !has {
		ledger.EnsureUser("alice", map[string]float64{"USDT": 10000, "BTC": 1})
		ledger.EnsureUser("bob", map[string]float64{"USDT": 10000, "BTC": 1})
		log.Info("seeded demo balances", "users", []string{"alice", "bob"})
	}

	// [Kafka] 有 brokers 才启用；没有则 Noop（本地可不启 Kafka）
	var pub mq.TradePublisher = mq.NoopPublisher{}
	if len(cfg.KafkaBrokers) > 0 {
		kp, err := mq.NewKafkaPublisher(cfg.KafkaBrokers, cfg.KafkaTopic, log)
		if err != nil {
			log.Error("kafka publisher", "err", err)
			os.Exit(1)
		}
		pub = kp
		defer kp.Close()
		log.Info("kafka enabled", "brokers", cfg.KafkaBrokers, "topic", cfg.KafkaTopic)
	} else {
		log.Info("kafka disabled (set KAFKA_BROKERS to enable)")
	}

	authSvc := auth.New(cfg.JWTSecret, cfg.JWTHours, db)
	if err := authSvc.SeedDemoUsers(ctx); err != nil {
		log.Error("seed demo users", "err", err)
		os.Exit(1)
	}
	eng := engine.New("BTC-USDT", "BTC", "USDT", ledger, db, pub)
	apiServer := api.New(ledger, eng, rdb, authSvc)
	r := apiServer.Engine()

	web, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Error("static fs", "err", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(web))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	go func() {
		base := "http://localhost" + cfg.HTTPAddr
		log.Info("exchange started",
			"frontend", base+"/",
			"api", base+"/api/health",
			"login", "POST "+base+"/api/login  body: {\"user_id\":\"alice\",\"password\":\"alice\"}",
			"register", "POST "+base+"/api/register  body: {\"user_id\":\"carol\",\"password\":\"carol\"}",
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	// 优雅退出：等 Ctrl+C，再 Shutdown（学生产习惯）
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
	log.Info("bye")
}
