package account

import (
	"fmt"
	"sync"
)

// Ledger 简易账本：用户 -> 币种 -> 余额
// available = 可用；locked = 挂单冻结
type Ledger struct {
	mu       sync.Mutex
	balances map[string]map[string]*Balance // userID -> asset -> bal
}

type Balance struct {
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
}

func NewLedger() *Ledger {
	return &Ledger{balances: make(map[string]map[string]*Balance)}
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
	}
}

func (l *Ledger) GetBalances(userID string) map[string]Balance {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Balance)
	for asset, b := range l.balances[userID] {
		out[asset] = *b
	}
	return out
}

func (l *Ledger) Deposit(userID, asset string, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be > 0")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ensure(userID, asset)
	l.balances[userID][asset].Available += amount
	return nil
}

// Lock 下单时冻结资金
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
	return nil
}

// Unlock 撤单时解冻
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

// SettleBuy 买方成交：花掉冻结的 quote，得到 base
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

// SettleSell 卖方成交：花掉冻结的 base，得到 quote
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

func (l *Ledger) ensure(userID, asset string) {
	if l.balances[userID] == nil {
		l.balances[userID] = make(map[string]*Balance)
	}
	if l.balances[userID][asset] == nil {
		l.balances[userID][asset] = &Balance{}
	}
}
