// Package cache：Redis 缓存层
// 用途：热点余额缓存、简易 API 限流
//
// 标记约定：
//   [Redis]         Redis 相关
//   [Redis:READ]    读缓存
//   [Redis:WRITE]   写缓存 / 限流计数
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

type BalanceCache struct {
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
}

// Open [Redis] 连接并 Ping
func Open(addr, password string) (*Redis, error) {
	c := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis: %w", err)
	}
	return &Redis{client: c}, nil
}

func (r *Redis) Close() error { return r.client.Close() }

func balKey(userID string) string { return "bal:" + userID }

// SetBalances [Redis:WRITE] 覆盖写入某用户全部余额缓存
func (r *Redis) SetBalances(ctx context.Context, userID string, bals map[string]BalanceCache) error {
	if r == nil {
		return nil
	}
	key := balKey(userID)
	pipe := r.client.Pipeline()
	pipe.Del(ctx, key)
	for asset, b := range bals {
		raw, _ := json.Marshal(b)
		pipe.HSet(ctx, key, asset, raw)
	}
	pipe.Expire(ctx, key, 10*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

// GetBalances [Redis:READ] 读缓存；miss 时 ok=false
func (r *Redis) GetBalances(ctx context.Context, userID string) (map[string]BalanceCache, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	m, err := r.client.HGetAll(ctx, balKey(userID)).Result()
	if err != nil {
		return nil, false, err
	}
	if len(m) == 0 {
		return nil, false, nil
	}
	out := make(map[string]BalanceCache, len(m))
	for asset, raw := range m {
		var b BalanceCache
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			continue
		}
		out[asset] = b
	}
	return out, true, nil
}

// InvalidateBalances [Redis:WRITE] 删除用户余额缓存
func (r *Redis) InvalidateBalances(ctx context.Context, userID string) error {
	if r == nil {
		return nil
	}
	return r.client.Del(ctx, balKey(userID)).Err()
}

// Allow [Redis:WRITE] 简易限流：每 window 内最多 limit 次
func (r *Redis) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if r == nil {
		return true, nil
	}
	k := "rl:" + key
	n, err := r.client.Incr(ctx, k).Result() // [Redis:WRITE]
	if err != nil {
		return true, err // 限流失败时放行，避免拖垮主流程
	}
	if n == 1 {
		_ = r.client.Expire(ctx, k, window).Err() // [Redis:WRITE]
	}
	return n <= limit, nil
}
