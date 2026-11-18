package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/demo/dex/internal/types"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	mu         sync.RWMutex
	users      map[string]*types.User
	emailIndex map[string]string
	walletIdx  map[string]string
	codeIndex  map[string]string
	balances   map[string]map[string]decimal.Decimal // user -> asset -> avail
	locked     map[string]map[string]decimal.Decimal
	orders     map[string]*types.Order
	positions  map[string]*types.Position
	trades     []*types.Trade
	ledgers    []*types.Ledger
	points     map[string]decimal.Decimal
	volume     map[string]decimal.Decimal
	rebates    map[string]decimal.Decimal
}

func New() *Store {
	s := &Store{
		users:      map[string]*types.User{},
		emailIndex: map[string]string{},
		walletIdx:  map[string]string{},
		codeIndex:  map[string]string{},
		balances:   map[string]map[string]decimal.Decimal{},
		locked:     map[string]map[string]decimal.Decimal{},
		orders:     map[string]*types.Order{},
		positions:  map[string]*types.Position{},
		points:     map[string]decimal.Decimal{},
		volume:     map[string]decimal.Decimal{},
		rebates:    map[string]decimal.Decimal{},
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	hash, _ := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	demo := &types.User{
		ID: "u_demo", Email: "demo@tf.dex", PasswordHash: string(hash),
		DisplayName: "Demo Trader", ReferralCode: "TFDEMO", CreatedAt: time.Now().UTC(),
	}
	admin := &types.User{
		ID: "u_admin", Email: "admin@tf.dex", PasswordHash: string(adminHash),
		DisplayName: "TF Admin", ReferralCode: "TFADMIN", IsAdmin: true, CreatedAt: time.Now().UTC(),
	}
	s.users[demo.ID] = demo
	s.users[admin.ID] = admin
	s.emailIndex[demo.Email] = demo.ID
	s.emailIndex[admin.Email] = admin.ID
	s.codeIndex[demo.ReferralCode] = demo.ID
	s.codeIndex[admin.ReferralCode] = admin.ID
	s.ensureBal(demo.ID)
	s.ensureBal(admin.ID)
	s.balances[demo.ID]["USDT"] = decimal.RequireFromString("100000")
	s.balances[demo.ID]["BTC"] = decimal.RequireFromString("1.5")
	s.balances[admin.ID]["USDT"] = decimal.RequireFromString("1000000")
}

func (s *Store) ensureBal(userID string) {
	if s.balances[userID] == nil {
		s.balances[userID] = map[string]decimal.Decimal{"USDT": decimal.Zero, "BTC": decimal.Zero, "ETH": decimal.Zero, "SOL": decimal.Zero}
	}
	if s.locked[userID] == nil {
		s.locked[userID] = map[string]decimal.Decimal{"USDT": decimal.Zero, "BTC": decimal.Zero, "ETH": decimal.Zero, "SOL": decimal.Zero}
	}
}

func ID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (s *Store) CreateUser(email, password, name, invite string) (*types.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.emailIndex[email]; ok {
		return nil, fmt.Errorf("email already registered")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &types.User{
		ID: ID("u_"), Email: email, PasswordHash: string(hash),
		DisplayName: name, ReferralCode: ID("TF")[:10], CreatedAt: time.Now().UTC(),
	}
	if name == "" {
		u.DisplayName = email
	}
	if invite != "" {
		if inviter, ok := s.codeIndex[invite]; ok {
			u.InvitedBy = inviter
			s.points[inviter] = s.points[inviter].Add(decimal.NewFromInt(50))
			s.points[u.ID] = s.points[u.ID].Add(decimal.NewFromInt(20))
		}
	}
	s.users[u.ID] = u
	s.emailIndex[email] = u.ID
	s.codeIndex[u.ReferralCode] = u.ID
	s.ensureBal(u.ID)
	s.balances[u.ID]["USDT"] = decimal.RequireFromString("10000")
	return u, nil
}

func (s *Store) Login(email, password string) (*types.User, error) {
	s.mu.RLock()
	id, ok := s.emailIndex[email]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid credentials")
	}
	s.mu.RLock()
	u := s.users[id]
	s.mu.RUnlock()
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return u, nil
}

func (s *Store) UpsertWallet(address, chain, invite string) *types.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := chain + ":" + address
	if id, ok := s.walletIdx[key]; ok {
		return s.users[id]
	}
	u := &types.User{
		ID: ID("u_"), Email: address[:6] + "@wallet.tf", DisplayName: address[:8] + "…",
		ReferralCode: ID("TF")[:10], CreatedAt: time.Now().UTC(),
	}
	if chain == "sol" {
		u.WalletSOL = address
	} else {
		u.WalletEVM = address
	}
	if invite != "" {
		if inviter, ok := s.codeIndex[invite]; ok {
			u.InvitedBy = inviter
			s.points[inviter] = s.points[inviter].Add(decimal.NewFromInt(50))
		}
	}
	s.users[u.ID] = u
	s.walletIdx[key] = u.ID
	s.codeIndex[u.ReferralCode] = u.ID
	s.ensureBal(u.ID)
	s.balances[u.ID]["USDT"] = decimal.RequireFromString("10000")
	return u
}

func (s *Store) User(id string) *types.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[id]
}

func (s *Store) Balances(userID string) []types.Balance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureBal(userID)
	out := []types.Balance{}
	for asset, avail := range s.balances[userID] {
		out = append(out, types.Balance{
			Asset: asset, Available: avail.String(), Locked: s.locked[userID][asset].String(),
		})
	}
	return out
}

func (s *Store) Credit(userID, asset string, amount decimal.Decimal, kind, ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureBal(userID)
	s.balances[userID][asset] = s.balances[userID][asset].Add(amount)
	s.ledgers = append(s.ledgers, &types.Ledger{
		ID: ID("l_"), UserID: userID, Asset: asset, Amount: amount.String(), Kind: kind, Ref: ref, CreatedAt: time.Now().UTC(),
	})
}

func (s *Store) Debit(userID, asset string, amount decimal.Decimal, kind, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureBal(userID)
	if s.balances[userID][asset].LessThan(amount) {
		return fmt.Errorf("insufficient %s", asset)
	}
	s.balances[userID][asset] = s.balances[userID][asset].Sub(amount)
	s.ledgers = append(s.ledgers, &types.Ledger{
		ID: ID("l_"), UserID: userID, Asset: asset, Amount: amount.Neg().String(), Kind: kind, Ref: ref, CreatedAt: time.Now().UTC(),
	})
	return nil
}

func (s *Store) Lock(userID, asset string, amount decimal.Decimal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureBal(userID)
	if s.balances[userID][asset].LessThan(amount) {
		return fmt.Errorf("insufficient margin")
	}
	s.balances[userID][asset] = s.balances[userID][asset].Sub(amount)
	s.locked[userID][asset] = s.locked[userID][asset].Add(amount)
	return nil
}

func (s *Store) Unlock(userID, asset string, amount decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureBal(userID)
	if s.locked[userID][asset].LessThan(amount) {
		amount = s.locked[userID][asset]
	}
	s.locked[userID][asset] = s.locked[userID][asset].Sub(amount)
	s.balances[userID][asset] = s.balances[userID][asset].Add(amount)
}

func (s *Store) PutOrder(o *types.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
}

func (s *Store) Order(id string) *types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orders[id]
}

func (s *Store) UserOrders(userID string) []*types.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.Order{}
	for _, o := range s.orders {
		if o.UserID == userID {
			cp := *o
			out = append(out, &cp)
		}
	}
	return out
}

func (s *Store) PutPosition(p *types.Position) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.positions[p.ID] = p
}

func (s *Store) DeletePosition(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.positions, id)
}

func (s *Store) UserPositions(userID string) []*types.Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.Position{}
	for _, p := range s.positions {
		if p.UserID == userID {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out
}

func (s *Store) AllPositions() []*types.Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.Position{}
	for _, p := range s.positions {
		cp := *p
		out = append(out, &cp)
	}
	return out
}

func (s *Store) AddTrade(t *types.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trades = append(s.trades, t)
	if len(s.trades) > 2000 {
		s.trades = s.trades[len(s.trades)-2000:]
	}
}

func (s *Store) RecentTrades(symbol string, n int) []*types.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.Trade{}
	for i := len(s.trades) - 1; i >= 0 && len(out) < n; i-- {
		if symbol == "" || s.trades[i].Symbol == symbol {
			out = append(out, s.trades[i])
		}
	}
	return out
}

func (s *Store) UserTrades(userID string, n int) []*types.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.Trade{}
	for i := len(s.trades) - 1; i >= 0 && len(out) < n; i-- {
		t := s.trades[i]
		if t.BuyerID == userID || t.SellerID == userID {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) Ledgers(userID string) []*types.Ledger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.Ledger{}
	for i := len(s.ledgers) - 1; i >= 0; i-- {
		if s.ledgers[i].UserID == userID {
			out = append(out, s.ledgers[i])
		}
	}
	return out
}

func (s *Store) AddPoints(userID string, volume, pts decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points[userID] = s.points[userID].Add(pts)
	s.volume[userID] = s.volume[userID].Add(volume)
}

func (s *Store) AddRebate(userID string, amount decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebates[userID] = s.rebates[userID].Add(amount)
	s.ensureBal(userID)
	s.balances[userID]["USDT"] = s.balances[userID]["USDT"].Add(amount)
}

func (s *Store) PointsOf(userID string) types.Points {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return types.Points{UserID: userID, Balance: s.points[userID].String(), VolumeUSD: s.volume[userID].String()}
}

func (s *Store) ReferralOf(u *types.User) types.ReferralStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, x := range s.users {
		if x.InvitedBy == u.ID {
			n++
		}
	}
	return types.ReferralStats{Code: u.ReferralCode, Invitees: n, RebateUSDT: s.rebates[u.ID].String(), Points: s.points[u.ID].String()}
}

func (s *Store) Stats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tv := decimal.Zero
	for _, v := range s.volume {
		tv = tv.Add(v)
	}
	return map[string]any{
		"users": len(s.users), "orders": len(s.orders), "positions": len(s.positions),
		"trades": len(s.trades), "volume_usd": tv.String(),
	}
}

func (s *Store) Users() []*types.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.User{}
	for _, u := range s.users {
		cp := *u
		out = append(out, &cp)
	}
	return out
}
