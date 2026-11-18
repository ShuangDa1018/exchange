// 排行榜微服务入口（独立进程）
// 依赖：docker compose up -d（postgres / redis / redpanda）
// 运行：cd exchange && go run ./cmd/ranking
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/config"
	"github.com/demo/exchange/internal/ranking"
	"github.com/demo/exchange/internal/store"
)

func main() {
	cfg := config.Load()
	if cfg.HTTPAddr == ":8080" {
		cfg.HTTPAddr = ":8081" // 与交易服务错开端口
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rdb, err := cache.Open(cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Error("redis unavailable", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	board := ranking.NewBoard(rdb)

	if cfg.DatabaseURL != "" {
		db, err := store.OpenReadOnly(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Warn("postgres unavailable, skip backfill", "err", err)
		} else {
			defer db.Close()
			if err := board.Backfill(ctx, db); err != nil {
				log.Error("backfill", "err", err)
			} else {
				log.Info("postgres backfill done")
			}
		}
	}

	if err := board.RefreshAllEquity(ctx); err != nil {
		log.Warn("equity refresh", "err", err)
	}

	if len(cfg.KafkaBrokers) > 0 {
		go ranking.ConsumeTrades(ctx, cfg.KafkaBrokers, cfg.KafkaTopic, "ranking", board, log)
	} else {
		log.Info("kafka disabled (set KAFKA_BROKERS to enable live ranking)")
	}

	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = board.RefreshAllEquity(ctx)
			}
		}
	}()

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: ranking.NewHTTP(board)}
	go func() {
		log.Info("ranking started", "addr", "http://localhost"+cfg.HTTPAddr, "leaderboard", "/api/leaderboard")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()
	shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = srv.Shutdown(shutdownCtx)
}
