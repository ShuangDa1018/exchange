// Package store：PostgreSQL 持久化（用户 / 余额 / 流水 / 订单 / 成交）
//
// 标记约定（全文可搜）：
//
//	[DB]         Postgres 相关
//	[DB:READ]    查询
//	[DB:WRITE]   插入 / 更新
//	[DB:MIGRATE] 迁移建表
//
// SQL 文件：
//
//	migrations/*.sql     改表结构
//	store/queries/*.sql  日常增删改查（本包 embed）
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/demo/exchange/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")

type DB struct {
	pool *pgxpool.Pool
}

// Open [DB] 连接 Postgres，并执行 migrations
func Open(ctx context.Context, databaseURL string, migrationsFS fs.FS) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("连接 Postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Ping Postgres: %w", err)
	}
	db := &DB{pool: pool}
	// [DB:MIGRATE] 跑 /migrations/*.sql
	if err := db.RunMigrations(ctx, migrationsFS); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return db, nil
}

func (db *DB) Close() { db.pool.Close() }

// OpenReadOnly [DB] 只连库、不跑 migrations（给 ranking 等下游微服务回放成交/余额）
func OpenReadOnly(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("连接 Postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Ping Postgres: %w", err)
	}
	return &DB{pool: pool}, nil
}

type BalanceRow struct {
	UserID    string
	Asset     string
	Available float64
	Locked    float64
}

// UpsertBalance [DB:WRITE] → queries/upsert_balance.sql
func (db *DB) UpsertBalance(ctx context.Context, userID, asset string, available, locked float64) error {
	_, err := db.pool.Exec(ctx, sqlUpsertBalance, userID, asset, available, locked)
	return err
}

// LoadBalances [DB:READ] → queries/load_balances.sql
func (db *DB) LoadBalances(ctx context.Context) ([]BalanceRow, error) {
	rows, err := db.pool.Query(ctx, sqlLoadBalances)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BalanceRow
	for rows.Next() {
		var r BalanceRow
		if err := rows.Scan(&r.UserID, &r.Asset, &r.Available, &r.Locked); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// HasAnyBalance [DB:READ] → queries/has_any_balance.sql
func (db *DB) HasAnyBalance(ctx context.Context) (bool, error) {
	var n int
	err := db.pool.QueryRow(ctx, sqlHasAnyBalance).Scan(&n)
	return n > 0, err
}

type LedgerEntry struct {
	ID             int64     `json:"id"`
	UserID         string    `json:"user_id"`
	Asset          string    `json:"asset"`
	DeltaAvailable float64   `json:"delta_available"`
	DeltaLocked    float64   `json:"delta_locked"`
	Reason         string    `json:"reason"`
	RefID          string    `json:"ref_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// AppendLedger [DB:WRITE] → queries/append_ledger.sql
func (db *DB) AppendLedger(ctx context.Context, userID, asset string, dAvail, dLocked float64, reason, refID string) error {
	_, err := db.pool.Exec(ctx, sqlAppendLedger, userID, asset, dAvail, dLocked, reason, refID)
	return err
}

// ListLedger [DB:READ] → queries/list_ledger.sql
func (db *DB) ListLedger(ctx context.Context, userID string, limit int) ([]LedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.pool.Query(ctx, sqlListLedger, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Asset, &e.DeltaAvailable, &e.DeltaLocked, &e.Reason, &e.RefID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpsertOrder [DB:WRITE] → queries/upsert_order.sql
func (db *DB) UpsertOrder(ctx context.Context, o *types.Order) error {
	_, err := db.pool.Exec(ctx, sqlUpsertOrder,
		o.ID, o.UserID, o.Symbol, string(o.Side), o.Price, o.Quantity, o.Filled, string(o.Status), o.CreatedAt.UTC())
	return err
}

// LoadOrders [DB:READ] → queries/load_orders.sql
func (db *DB) LoadOrders(ctx context.Context) ([]*types.Order, error) {
	rows, err := db.pool.Query(ctx, sqlLoadOrders)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*types.Order
	for rows.Next() {
		var o types.Order
		var side, status string
		if err := rows.Scan(&o.ID, &o.UserID, &o.Symbol, &side, &o.Price, &o.Quantity, &o.Filled, &status, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.Side = types.Side(side)
		o.Status = types.OrderStatus(status)
		out = append(out, &o)
	}
	return out, rows.Err()
}

// InsertTrade [DB:WRITE] → queries/insert_trade.sql
func (db *DB) InsertTrade(ctx context.Context, t *types.Trade) error {
	_, err := db.pool.Exec(ctx, sqlInsertTrade,
		t.ID, t.Symbol, t.Price, t.Quantity, t.BuyOrder, t.SellOrder, t.BuyerID, t.SellerID, t.CreatedAt.UTC())
	return err
}

// LoadTrades [DB:READ] → queries/load_trades.sql
func (db *DB) LoadTrades(ctx context.Context) ([]*types.Trade, error) {
	rows, err := db.pool.Query(ctx, sqlLoadTrades)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*types.Trade
	for rows.Next() {
		var t types.Trade
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Price, &t.Quantity, &t.BuyOrder, &t.SellOrder, &t.BuyerID, &t.SellerID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// CreateUser [DB:WRITE] → queries/create_user.sql
func (db *DB) CreateUser(ctx context.Context, userID, passwordHash string) error {
	var id string
	err := db.pool.QueryRow(ctx, sqlCreateUser, userID, passwordHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserExists
	}
	return err
}

// GetUserPassword [DB:READ] → queries/get_user_password.sql
func (db *DB) GetUserPassword(ctx context.Context, userID string) (string, error) {
	var hash string
	err := db.pool.QueryRow(ctx, sqlGetUserPassword, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	return hash, err
}

// EnsureUser [DB:WRITE] → queries/ensure_user.sql（种子用户，已存在则跳过）
func (db *DB) EnsureUser(ctx context.Context, userID, passwordHash string) error {
	_, err := db.pool.Exec(ctx, sqlEnsureUser, userID, passwordHash)
	return err
}
