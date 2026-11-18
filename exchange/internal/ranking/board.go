// Package ranking：排行榜微服务的核心（成交额 / 总资产）
//
// 数据流：
//
//	exchange 撮合成交 → Kafka topic trades → 本服务消费
//	启动时 Postgres 回放 trades/balances，避免 Redis 空窗
//	Redis ZSET 存排行；余额 Hash 约定与 account 相同：bal:{userID}
package ranking

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/store"
	"github.com/demo/exchange/internal/types"
	"github.com/redis/go-redis/v9"
)

const (
	keyVol    = "rank:vol"
	keyCount  = "rank:count"
	keyEquity = "rank:equity"
	keyPrice  = "rank:last_price"
	keySeen   = "rank:seen"
	defaultPx = 100.0
)

// Row 排行榜一行
type Row struct {
	Rank   int     `json:"rank"`
	UserID string  `json:"user_id"`
	Volume float64 `json:"volume"`
	Trades int64   `json:"trades"`
	Equity float64 `json:"equity"`
}

type Board struct {
	rdb *redis.Client
}

func NewBoard(rdb *cache.Redis) *Board {
	if rdb == nil {
		return &Board{}
	}
	return &Board{rdb: rdb.Client()}
}

// Equity 总资产 ≈ USDT + BTC * 最新价
func Equity(usdt, btc, lastPrice float64) float64 {
	if lastPrice <= 0 {
		lastPrice = defaultPx
	}
	return usdt + btc*lastPrice
}

func notional(t *types.Trade) float64 {
	if t == nil {
		return 0
	}
	return t.Price * t.Quantity
}

func parties(t *types.Trade) []string {
	if t.BuyerID == t.SellerID {
		return []string{t.BuyerID}
	}
	return []string{t.BuyerID, t.SellerID}
}

// ApplyTrade [Redis:WRITE] 计入成交额；同一 trade id 只处理一次
func (b *Board) ApplyTrade(ctx context.Context, t *types.Trade) error {
	if b == nil || b.rdb == nil || t == nil || t.ID == 0 {
		return nil
	}
	added, err := b.rdb.SAdd(ctx, keySeen, strconv.FormatInt(t.ID, 10)).Result()
	if err != nil {
		return err
	}
	if added == 0 {
		return nil
	}
	n := notional(t)
	pipe := b.rdb.Pipeline()
	for _, uid := range parties(t) {
		if uid == "" {
			continue
		}
		pipe.ZIncrBy(ctx, keyVol, n, uid)
		pipe.ZIncrBy(ctx, keyCount, 1, uid)
	}
	pipe.Set(ctx, keyPrice, t.Price, 0)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	for _, uid := range parties(t) {
		_ = b.refreshEquity(ctx, uid)
	}
	return nil
}

// LastPrice [Redis:READ]
func (b *Board) LastPrice(ctx context.Context) float64 {
	if b == nil || b.rdb == nil {
		return defaultPx
	}
	px, err := b.rdb.Get(ctx, keyPrice).Float64()
	if err != nil || px <= 0 {
		return defaultPx
	}
	return px
}

func (b *Board) refreshEquity(ctx context.Context, userID string) error {
	m, err := b.rdb.HGetAll(ctx, "bal:"+userID).Result()
	if err != nil || len(m) == 0 {
		return err
	}
	var usdt, btc float64
	for asset, raw := range m {
		var row cache.BalanceCache
		if json.Unmarshal([]byte(raw), &row) != nil {
			continue
		}
		total := row.Available + row.Locked
		switch strings.ToUpper(asset) {
		case "USDT":
			usdt = total
		case "BTC":
			btc = total
		}
	}
	return b.rdb.ZAdd(ctx, keyEquity, redis.Z{Score: Equity(usdt, btc, b.LastPrice(ctx)), Member: userID}).Err()
}

// RefreshAllEquity [Redis] 扫 bal:* 重算总资产（入金/未成交用户也会上榜）
func (b *Board) RefreshAllEquity(ctx context.Context) error {
	if b == nil || b.rdb == nil {
		return nil
	}
	var cursor uint64
	for {
		keys, next, err := b.rdb.Scan(ctx, cursor, "bal:*", 50).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			uid := strings.TrimPrefix(key, "bal:")
			if uid == "" || uid == key {
				continue
			}
			_ = b.refreshEquity(ctx, uid)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Backfill [DB:READ] 回放成交和余额，再写入 Redis
func (b *Board) Backfill(ctx context.Context, db *store.DB) error {
	if db == nil {
		return nil
	}
	trades, err := db.LoadTrades(ctx)
	if err != nil {
		return err
	}
	for _, t := range trades {
		if err := b.ApplyTrade(ctx, t); err != nil {
			return err
		}
	}
	rows, err := db.LoadBalances(ctx)
	if err != nil {
		return err
	}
	px := b.LastPrice(ctx)
	byUser := map[string]map[string]float64{}
	for _, r := range rows {
		if byUser[r.UserID] == nil {
			byUser[r.UserID] = map[string]float64{}
		}
		byUser[r.UserID][strings.ToUpper(r.Asset)] += r.Available + r.Locked
	}
	for uid, assets := range byUser {
		eq := Equity(assets["USDT"], assets["BTC"], px)
		if err := b.rdb.ZAdd(ctx, keyEquity, redis.Z{Score: eq, Member: uid}).Err(); err != nil {
			return err
		}
	}
	return nil
}

// Top [Redis:READ] sort=volume|equity
func (b *Board) Top(ctx context.Context, sortBy string, limit int) ([]Row, float64, error) {
	if b == nil || b.rdb == nil {
		return []Row{}, defaultPx, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	key := keyVol
	if sortBy == "equity" {
		key = keyEquity
	}
	zs, err := b.rdb.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, 0, err
	}
	px := b.LastPrice(ctx)
	out := make([]Row, 0, len(zs))
	for i, z := range zs {
		uid, _ := z.Member.(string)
		vol, _ := b.rdb.ZScore(ctx, keyVol, uid).Result()
		cnt, _ := b.rdb.ZScore(ctx, keyCount, uid).Result()
		eq, _ := b.rdb.ZScore(ctx, keyEquity, uid).Result()
		row := Row{
			Rank:   i + 1,
			UserID: uid,
			Volume: vol,
			Trades: int64(cnt),
			Equity: eq,
		}
		if sortBy == "equity" {
			row.Equity = z.Score
		} else {
			row.Volume = z.Score
		}
		out = append(out, row)
	}
	return out, px, nil
}
