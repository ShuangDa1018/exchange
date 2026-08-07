// Package account：用户余额账本
// 内存热更新 + Postgres 落盘 + Redis 缓存
//
// 标记约定：
//   [DB:READ] / [DB:WRITE]  Postgres
//   [Redis:READ] / [Redis:WRITE]  缓存
package account

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/store"
)

type Ledger struct {
	mu       sync.Mutex
	balances map[string]map[string]*Balance
	db       *store.DB    // [DB] Postgres
	rdb      *cache.Redis // [Redis]
}

type Balance struct {
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
}

func NewLedger(db *store.DB, rdb *cache.Redis) *Ledger {
	l := &Ledger{
		balances: make(map[string]map[string]*Balance),
		db:       db,
		rdb:      rdb,
	}
	if db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// [DB:READ] 启动时从 Postgres 加载余额
		if err := l.load(ctx); err != nil {
			log.Printf("加载余额失败: %v", err)
		}
	}
	return l
}

// load [DB:READ] + [Redis:WRITE] 读库并预热缓存
func (l *Ledger) load(ctx context.Context) error {
	rows, err := l.db.LoadBalances(ctx) // [DB:READ]
	if err != nil {
		return err
	}
	for _, r := range rows {
		l.ensure(r.UserID, r.Asset)
		l.balances[r.UserID][r.Asset] = &Balance{Available: r.Available, Locked: r.Locked}
	}
	for userID, assets := range l.balances {
		l.syncRedis(userID, assets) // [Redis:WRITE]
	}
	return nil
}

func (l *Ledger) EnsureUser(userID string, seed map[string]float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.balances[userID]; ok {
		return
	}
	l.balances[userID] = make(map[string]*Balance)
	for asset, amount := range seed {
		l.balances[userID][asset] = &Balance{Available: amount}
		l.persist(userID, asset, amount, 0, "seed", "") // [DB:WRITE] + [Redis:WRITE]
	}
}

// GetBalances [Redis:READ] 优先；miss 则读内存并 [Redis:WRITE] 回填
func (l *Ledger) GetBalances(userID string) map[string]Balance {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if cached, ok, err := l.rdb.GetBalances(ctx, userID); err == nil && ok { // [Redis:READ]
		out := make(map[string]Balance, len(cached))
		for asset, b := range cached {
			out[asset] = Balance{Available: b.Available, Locked: b.Locked}
		}
		return out
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Balance)
	for asset, b := range l.balances[userID] {
		out[asset] = *b
	}
	l.syncRedis(userID, l.balances[userID]) // [Redis:WRITE]
	return out
}

// ListLedger [DB:READ] 查余额流水
func (l *Ledger) ListLedger(userID string, limit int) ([]store.LedgerEntry, error) {
	if l.db == nil {
		return nil, fmt.Errorf("未启用数据库")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return l.db.ListLedger(ctx, userID, limit) // [DB:READ]
}

func (l *Ledger) Deposit(userID, asset string, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be > 0")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(userID, asset)
	l.balances[userID][asset].Available += amount
	l.persist(userID, asset, amount, 0, "deposit", "") // [DB:WRITE]
	return nil
}

func (l *Ledger) Lock(userID, asset string, amount float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(userID, asset)
	b := l.balances[userID][asset]
	if b.Available < amount {
		return fmt.Errorf("余额不足: %s 可用 %.4f, 需要 %.4f", asset, b.Available, amount)
	}
	b.Available -= amount
	b.Locked += amount
	l.persist(userID, asset, -amount, amount, "lock", "") // [DB:WRITE]
	return nil
}

func (l *Ledger) Unlock(userID, asset string, amount float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(userID, asset)
	b := l.balances[userID][asset]
	if amount > b.Locked {
		amount = b.Locked
	}
	b.Locked -= amount
	b.Available += amount
	l.persist(userID, asset, amount, -amount, "unlock", "") // [DB:WRITE]
}

func (l *Ledger) SettleBuy(buyer, base, quote string, baseQty, quoteCost float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(buyer, base)
	l.ensure(buyer, quote)
	l.balances[buyer][quote].Locked -= quoteCost
	if l.balances[buyer][quote].Locked < 0 {
		l.balances[buyer][quote].Locked = 0
	}
	l.balances[buyer][base].Available += baseQty
	l.persist(buyer, quote, 0, -quoteCost, "settle_buy", "") // [DB:WRITE]
	l.persist(buyer, base, baseQty, 0, "settle_buy", "")     // [DB:WRITE]
}

func (l *Ledger) SettleSell(seller, base, quote string, baseQty, quoteProceeds float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(seller, base)
	l.ensure(seller, quote)
	l.balances[seller][base].Locked -= baseQty
	if l.balances[seller][base].Locked < 0 {
		l.balances[seller][base].Locked = 0
	}
	l.balances[seller][quote].Available += quoteProceeds
	l.persist(seller, base, 0, -baseQty, "settle_sell", "")         // [DB:WRITE]
	l.persist(seller, quote, quoteProceeds, 0, "settle_sell", "") // [DB:WRITE]
}

func (l *Ledger) ensure(userID, asset string) {
	if l.balances[userID] == nil {
		l.balances[userID] = make(map[string]*Balance)
	}
	if l.balances[userID][asset] == nil {
		l.balances[userID][asset] = &Balance{}
	}
}

// persist [DB:WRITE] 余额快照 + 流水；[Redis:WRITE] 刷新缓存
func (l *Ledger) persist(userID, asset string, dAvail, dLocked float64, reason, refID string) {
	b := l.balances[userID][asset]
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if l.db != nil {
		if err := l.db.UpsertBalance(ctx, userID, asset, b.Available, b.Locked); err != nil { // [DB:WRITE]
			log.Printf("Postgres 写余额失败 %s/%s: %v", userID, asset, err)
		}
		if err := l.db.AppendLedger(ctx, userID, asset, dAvail, dLocked, reason, refID); err != nil { // [DB:WRITE]
			log.Printf("Postgres 写流水失败 %s/%s: %v", userID, asset, err)
		}
	}
	l.syncRedis(userID, l.balances[userID]) // [Redis:WRITE]
}

// syncRedis [Redis:WRITE] 覆盖写用户余额 Hash
func (l *Ledger) syncRedis(userID string, assets map[string]*Balance) {
	if l.rdb == nil || assets == nil {
		return
	}
	m := make(map[string]cache.BalanceCache, len(assets))
	for asset, b := range assets {
		m[asset] = cache.BalanceCache{Available: b.Available, Locked: b.Locked}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := l.rdb.SetBalances(ctx, userID, m); err != nil { // [Redis:WRITE]
		log.Printf("Redis 写余额缓存失败 %s: %v", userID, err)
	}
}
