package ranking

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/demo/exchange/internal/types"
	"github.com/segmentio/kafka-go"
)

// ConsumeTrades [Kafka] 消费 trades，写入排行榜
func ConsumeTrades(ctx context.Context, brokers []string, topic, group string, board *Board, log *slog.Logger) {
	if topic == "" {
		topic = "trades"
	}
	if group == "" {
		group = "ranking"
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        group,
		StartOffset:    kafka.LastOffset, // 历史由 Postgres 回放，避免和 Redis 去重前双计
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})
	defer r.Close()
	log.Info("kafka consumer started", "topic", topic, "group", group, "brokers", brokers)
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("kafka read", "err", err)
			time.Sleep(time.Second)
			continue
		}
		var t types.Trade
		if err := json.Unmarshal(m.Value, &t); err != nil {
			log.Error("kafka trade json", "err", err)
			continue
		}
		if err := board.ApplyTrade(ctx, &t); err != nil {
			log.Error("apply trade", "err", err, "trade_id", t.ID)
		}
	}
}
