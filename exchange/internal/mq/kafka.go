// Package mq：消息队列（学习 Kafka：成交事件异步广播）
package mq

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/demo/exchange/internal/types"
	"github.com/segmentio/kafka-go"
)

// TradePublisher 成交事件发布接口（便于单测 mock / 无 Kafka 时用 Noop）
type TradePublisher interface {
	PublishTrade(ctx context.Context, t *types.Trade) error
	Close() error
}

type NoopPublisher struct{}

func (NoopPublisher) PublishTrade(context.Context, *types.Trade) error { return nil }
func (NoopPublisher) Close() error                                     { return nil }

// KafkaPublisher 把成交 JSON 发到 topic（默认 trades）
type KafkaPublisher struct {
	writer *kafka.Writer
	topic  string
	log    *slog.Logger
}

func NewKafkaPublisher(brokers []string, topic string, log *slog.Logger) (*KafkaPublisher, error) {
	if topic == "" {
		topic = "trades"
	}
	if log == nil {
		log = slog.Default()
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	return &KafkaPublisher{writer: w, topic: topic, log: log}, nil
}

func (p *KafkaPublisher) PublishTrade(ctx context.Context, t *types.Trade) error {
	body, err := json.Marshal(t)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(t.Symbol),
		Value: body,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		p.log.Error("kafka publish trade failed", "err", err, "trade_id", t.ID)
		return err
	}
	p.log.Info("kafka trade published", "topic", p.topic, "trade_id", t.ID, "price", t.Price, "qty", t.Quantity)
	return nil
}

func (p *KafkaPublisher) Close() error {
	if p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
