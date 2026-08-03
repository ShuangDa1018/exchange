// Package account：用户余额账本（≈ 前端的 wallet/store）
package account

import (
	"fmt"
	"sync"
)

// Ledger 内存账本
// 结构：userID -> asset -> Balance
// ≈ Record<userId, Record<asset, { available, locked }>>
type Ledger struct {
	mu       sync.Mutex                        // 互斥锁：防止并发请求同时改余额（前端单线程通常不需要）
	balances map[string]map[string]*Balance
}

// Balance 某个币种的余额
// available = 可下单；locked = 挂单冻结中（撤单会解冻）
type Balance struct {
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
}

// NewLedger 构造函数 ≈ function createLedger()
// make(...) 创建空 map，必须先 make，否则是 nil 不能写
func NewLedger() *Ledger {
	return &Ledger{balances: make(map[string]map[string]*Balance)}
}

// EnsureUser 用户不存在时创建，并给初始资金（演示用）
func (l *Ledger) EnsureUser(userID string, seed map[string]float64) {
	l.mu.Lock()         // 加锁
	defer l.mu.Unlock() // defer ≈ finally：函数返回前一定解锁
	// map 取值第二个返回值 ok：有没有这个 key（≈ hasOwnProperty）
	if _, ok := l.balances[userID]; ok {
		return
	}
	l.balances[userID] = make(map[string]*Balance)
	// for range map ≈ Object.entries(seed).forEach
	for asset, amount := range seed {
		l.balances[userID][asset] = &Balance{Available: amount}
	}
}

// GetBalances 查询余额（返回拷贝，避免外部直接改内部指针）
func (l *Ledger) GetBalances(userID string) map[string]Balance {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Balance)
	for asset, b := range l.balances[userID] {
		out[asset] = *b // *b 解引用：拷贝值，不是引用
	}
	return out
}

// Deposit 充值：available += amount
// 返回 error ≈ throw / Result；nil 表示成功
func (l *Ledger) Deposit(userID, asset string, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be > 0") // 创建错误信息
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(userID, asset)
	l.balances[userID][asset].Available += amount
	return nil
}

// Lock 下单时冻结：available → locked
func (l *Ledger) Lock(userID, asset string, amount float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(userID, asset)
	b := l.balances[userID][asset]
	if b.Available < amount {
		// %.4f ≈ toFixed(4)
		return fmt.Errorf("余额不足: %s 可用 %.4f, 需要 %.4f", asset, b.Available, amount)
	}
	b.Available -= amount
	b.Locked += amount
	return nil
}

// Unlock 撤单/差价退回：locked → available
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
}

// SettleBuy 买方成交结算：
// 花掉冻结的 quote（如 USDT），得到 base（如 BTC）
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
}

// SettleSell 卖方成交结算：
// 花掉冻结的 base（BTC），得到 quote（USDT）
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
}

// ensure 私有方法（小写开头 = 包外不可见，≈ #private）
// 保证 user/asset 这两层 map 都存在
func (l *Ledger) ensure(userID, asset string) {
	if l.balances[userID] == nil {
		l.balances[userID] = make(map[string]*Balance)
	}
	if l.balances[userID][asset] == nil {
		l.balances[userID][asset] = &Balance{} // 零值：Available=0, Locked=0
	}
}
