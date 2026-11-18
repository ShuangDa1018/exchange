package engine

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/demo/dex/internal/store"
	"github.com/demo/dex/internal/types"
	"github.com/shopspring/decimal"
)

type MarketState struct {
	Meta     types.Market
	Price    decimal.Decimal
	Mark     decimal.Decimal
	Index    decimal.Decimal
	Change   decimal.Decimal
	High     decimal.Decimal
	Low      decimal.Decimal
	Vol      decimal.Decimal
	OI       decimal.Decimal
	Klines   map[string][]types.Candle // interval -> candles
	Book     types.OrderBook
}

type Engine struct {
	mu      sync.RWMutex
	store   *store.Store
	markets map[string]*MarketState
	onTick  []func(symbol string, m *MarketState)
}

var intervals = []struct {
	Key string
	Dur time.Duration
}{
	{"1m", time.Minute},
	{"5m", 5 * time.Minute},
	{"15m", 15 * time.Minute},
	{"1h", time.Hour},
	{"4h", 4 * time.Hour},
	{"1d", 24 * time.Hour},
}

func New(st *store.Store) *Engine {
	e := &Engine{store: st, markets: map[string]*MarketState{}}
	seeds := []struct {
		sym, base string
		px        float64
	}{
		{"BTC-USDT", "BTC", 97250},
		{"ETH-USDT", "ETH", 3320},
		{"SOL-USDT", "SOL", 178.4},
	}
	now := time.Now().UTC()
	for _, s := range seeds {
		px := decimal.NewFromFloat(s.px)
		ms := &MarketState{
			Meta: types.Market{
				Symbol: s.sym, Base: s.base, Quote: "USDT", Type: "perp",
				TickSize: "0.1", LotSize: "0.001", MaxLeverage: 50,
				TakerFee: "0.0005", MakerFee: "0.0002", MaintMargin: "0.005",
				FundingRate: "0.0001",
			},
			Price: px, Mark: px, Index: px, High: px, Low: px,
			Klines: map[string][]types.Candle{},
		}
		for _, iv := range intervals {
			ms.Klines[iv.Key] = genHistory(px, iv.Dur, 400, now)
		}
		last := ms.Klines["1m"][len(ms.Klines["1m"])-1]
		ms.Price = dec(last.Close)
		ms.Mark = ms.Price
		ms.Index = ms.Price
		ms.High = dec(last.High)
		ms.Low = dec(last.Low)
		e.rebuildBook(ms)
		e.markets[s.sym] = ms
	}
	go e.loop()
	return e
}

func genHistory(start decimal.Decimal, step time.Duration, n int, end time.Time) []types.Candle {
	out := make([]types.Candle, 0, n)
	px := start.InexactFloat64()
	t := end.Add(-time.Duration(n) * step).Unix()
	stepSec := int64(step.Seconds())
	t = t - (t % stepSec)
	for i := 0; i < n; i++ {
		o := px
		drift := (rand.Float64() - 0.48) * px * 0.004
		c := math.Max(px+drift, px*0.2)
		h := math.Max(o, c) * (1 + rand.Float64()*0.002)
		l := math.Min(o, c) * (1 - rand.Float64()*0.002)
		vol := 10 + rand.Float64()*40
		out = append(out, types.Candle{
			Time: t, Open: f(o), High: f(h), Low: f(l), Close: f(c), Volume: f(vol),
		})
		px = c
		t += stepSec
	}
	return out
}

func (e *Engine) OnTick(fn func(string, *MarketState)) {
	e.mu.Lock()
	e.onTick = append(e.onTick, fn)
	e.mu.Unlock()
}

func (e *Engine) loop() {
	tk := time.NewTicker(400 * time.Millisecond)
	defer tk.Stop()
	for range tk.C {
		type snap struct {
			sym string
			m   MarketState
		}
		var events []snap
		e.mu.Lock()
		listeners := append([]func(string, *MarketState){}, e.onTick...)
		for sym, m := range e.markets {
			chg := (rand.Float64() - 0.5) * m.Price.InexactFloat64() * 0.0008
			next := m.Price.Add(decimal.NewFromFloat(chg))
			if next.LessThanOrEqual(decimal.Zero) {
				next = m.Price
			}
			m.Price = next
			m.Mark = next
			m.Index = next.Mul(decimal.RequireFromString("0.9998"))
			if next.GreaterThan(m.High) {
				m.High = next
			}
			if m.Low.IsZero() || next.LessThan(m.Low) {
				m.Low = next
			}
			vol := decimal.NewFromFloat(0.2 + rand.Float64()*1.4)
			m.Vol = m.Vol.Add(vol)
			e.appendKline(m, next, vol)
			e.rebuildBook(m)
			e.store.AddTrade(&types.Trade{
				ID: store.ID("t_"), Symbol: sym, Price: next.String(), Qty: vol.String(),
				Side: pickSide(), CreatedAt: time.Now().UTC(),
			})
			events = append(events, snap{sym: sym, m: *m})
		}
		e.mu.Unlock()
		for _, fn := range listeners {
			for i := range events {
				cp := events[i].m
				fn(events[i].sym, &cp)
			}
		}
		e.markPositions()
	}
}

func pickSide() types.Side {
	if rand.Intn(2) == 0 {
		return types.SideBuy
	}
	return types.SideSell
}

func (e *Engine) appendKline(m *MarketState, px, vol decimal.Decimal) {
	now := time.Now().UTC().Unix()
	for _, iv := range intervals {
		sec := int64(iv.Dur.Seconds())
		bucket := now - (now % sec)
		kl := m.Klines[iv.Key]
		if len(kl) == 0 {
			m.Klines[iv.Key] = []types.Candle{{Time: bucket, Open: px.String(), High: px.String(), Low: px.String(), Close: px.String(), Volume: vol.String()}}
			continue
		}
		last := kl[len(kl)-1]
		if last.Time == bucket {
			h := maxDec(dec(last.High), px)
			l := minDec(dec(last.Low), px)
			last.High, last.Low, last.Close = h.String(), l.String(), px.String()
			last.Volume = dec(last.Volume).Add(vol).String()
			kl[len(kl)-1] = last
			m.Klines[iv.Key] = kl
		} else {
			m.Klines[iv.Key] = append(kl, types.Candle{
				Time: bucket, Open: px.String(), High: px.String(), Low: px.String(), Close: px.String(), Volume: vol.String(),
			})
			if len(m.Klines[iv.Key]) > 1000 {
				m.Klines[iv.Key] = m.Klines[iv.Key][len(m.Klines[iv.Key])-1000:]
			}
		}
	}
}

func (e *Engine) rebuildBook(m *MarketState) {
	px := m.Mark.InexactFloat64()
	bids, asks := []types.BookLevel{}, []types.BookLevel{}
	for i := 1; i <= 16; i++ {
		bp := px * (1 - 0.00015*float64(i))
		ap := px * (1 + 0.00015*float64(i))
		q := 0.4 + rand.Float64()*3
		bids = append(bids, types.BookLevel{Price: f(bp), Qty: f(q)})
		asks = append(asks, types.BookLevel{Price: f(ap), Qty: f(q * 0.9)})
	}
	m.Book = types.OrderBook{Symbol: m.Meta.Symbol, Bids: bids, Asks: asks, Ts: time.Now().UnixMilli()}
}

func (e *Engine) Markets() []types.Market {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := []types.Market{}
	for _, m := range e.markets {
		meta := m.Meta
		meta.Price = m.Price.StringFixed(4)
		meta.MarkPrice = m.Mark.StringFixed(4)
		meta.IndexPrice = m.Index.StringFixed(4)
		if len(m.Klines["1h"]) > 24 {
			old := dec(m.Klines["1h"][len(m.Klines["1h"])-24].Close)
			if !old.IsZero() {
				meta.Change24h = m.Price.Sub(old).Div(old).StringFixed(6)
			}
		}
		meta.High24h = m.High.StringFixed(4)
		meta.Low24h = m.Low.StringFixed(4)
		meta.Volume24h = m.Vol.StringFixed(4)
		meta.OpenInterest = m.OI.StringFixed(4)
		out = append(out, meta)
	}
	return out
}

func (e *Engine) Market(symbol string) *MarketState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.markets[symbol]
}

func (e *Engine) Klines(symbol, interval string, limit int) []types.Candle {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := e.markets[symbol]
	if m == nil {
		return nil
	}
	kl := m.Klines[interval]
	if kl == nil {
		kl = m.Klines["1m"]
	}
	if limit <= 0 || limit > len(kl) {
		limit = len(kl)
	}
	return append([]types.Candle{}, kl[len(kl)-limit:]...)
}

func (e *Engine) Book(symbol string) types.OrderBook {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := e.markets[symbol]
	if m == nil {
		return types.OrderBook{}
	}
	return m.Book
}

func (e *Engine) Place(userID string, req PlaceReq) (*types.Order, *types.Position, error) {
	e.mu.Lock()
	m := e.markets[req.Symbol]
	e.mu.Unlock()
	if m == nil {
		return nil, nil, errf("unknown symbol")
	}
	qty, err := decimal.NewFromString(req.Qty)
	if err != nil || qty.LessThanOrEqual(decimal.Zero) {
		return nil, nil, errf("invalid qty")
	}
	lev := req.Leverage
	if lev < 1 {
		lev = 1
	}
	if lev > m.Meta.MaxLeverage {
		lev = m.Meta.MaxLeverage
	}
	px := m.Mark
	if req.Type == types.OrderLimit && req.Price != "" {
		p, err := decimal.NewFromString(req.Price)
		if err != nil {
			return nil, nil, errf("invalid price")
		}
		px = p
	}
	notional := px.Mul(qty)
	margin := notional.Div(decimal.NewFromInt(int64(lev)))
	fee := notional.Mul(decimal.RequireFromString(m.Meta.TakerFee))
	if err := e.store.Lock(userID, "USDT", margin.Add(fee)); err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	o := &types.Order{
		ID: store.ID("o_"), UserID: userID, Symbol: req.Symbol, Side: req.Side, PosSide: req.PosSide,
		Type: req.Type, Price: px.String(), Qty: qty.String(), FilledQty: qty.String(),
		Leverage: lev, Status: types.StatusFilled, Fee: fee.String(), ReduceOnly: req.ReduceOnly, CreatedAt: now, UpdatedAt: now,
	}
	e.store.PutOrder(o)
	tr := &types.Trade{
		ID: store.ID("t_"), Symbol: req.Symbol, Price: px.String(), Qty: qty.String(), Side: req.Side,
		CreatedAt: now, Fee: fee.String(),
	}
	if req.Side == types.SideBuy {
		tr.BuyerID = userID
	} else {
		tr.SellerID = userID
	}
	e.store.AddTrade(tr)
	e.store.AddPoints(userID, notional, notional.Div(decimal.NewFromInt(100)))
	if u := e.store.User(userID); u != nil && u.InvitedBy != "" {
		rebate := fee.Mul(decimal.RequireFromString("0.3"))
		e.store.AddRebate(u.InvitedBy, rebate)
	}

	pos := e.upsertPosition(userID, req.Symbol, req.PosSide, qty, px, lev, margin)
	e.mu.Lock()
	if mm := e.markets[req.Symbol]; mm != nil {
		mm.OI = mm.OI.Add(qty)
	}
	e.mu.Unlock()
	return o, pos, nil
}

func (e *Engine) upsertPosition(userID, symbol string, side types.PosSide, qty, px decimal.Decimal, lev int, margin decimal.Decimal) *types.Position {
	existing := e.store.UserPositions(userID)
	var cur *types.Position
	for _, p := range existing {
		if p.Symbol == symbol && p.Side == side {
			cur = p
			break
		}
	}
	if cur == nil {
		p := &types.Position{
			ID: store.ID("p_"), UserID: userID, Symbol: symbol, Side: side,
			Qty: qty.String(), EntryPrice: px.String(), MarkPrice: px.String(),
			Leverage: lev, Margin: margin.String(), UpdatedAt: time.Now().UTC(),
		}
		fillPnL(p, px)
		e.store.PutPosition(p)
		return p
	}
	oldQ := dec(cur.Qty)
	oldE := dec(cur.EntryPrice)
	newQ := oldQ.Add(qty)
	newE := oldE.Mul(oldQ).Add(px.Mul(qty)).Div(newQ)
	cur.Qty = newQ.String()
	cur.EntryPrice = newE.String()
	cur.Margin = dec(cur.Margin).Add(margin).String()
	cur.Leverage = lev
	fillPnL(cur, px)
	e.store.PutPosition(cur)
	return cur
}

func (e *Engine) ClosePosition(userID, posID string) (*types.Position, error) {
	var pos *types.Position
	for _, p := range e.store.UserPositions(userID) {
		if p.ID == posID {
			pos = p
			break
		}
	}
	if pos == nil {
		return nil, errf("position not found")
	}
	e.mu.RLock()
	m := e.markets[pos.Symbol]
	e.mu.RUnlock()
	if m == nil {
		return nil, errf("unknown symbol")
	}
	mark := m.Mark
	fillPnL(pos, mark)
	pnl := dec(pos.Unrealized)
	margin := dec(pos.Margin)
	e.store.Unlock(userID, "USDT", margin)
	if pnl.IsPositive() {
		e.store.Credit(userID, "USDT", pnl, "pnl", pos.ID)
	} else if pnl.IsNegative() {
		_ = e.store.Debit(userID, "USDT", pnl.Abs(), "pnl", pos.ID)
	}
	e.store.DeletePosition(pos.ID)
	pos.Qty = "0"
	pos.Unrealized = pnl.String()
	return pos, nil
}

func (e *Engine) markPositions() {
	for _, p := range e.store.AllPositions() {
		e.mu.RLock()
		m := e.markets[p.Symbol]
		e.mu.RUnlock()
		if m == nil {
			continue
		}
		fillPnL(p, m.Mark)
		p.MarkPrice = m.Mark.String()
		e.store.PutPosition(p)
		liq := dec(p.LiqPrice)
		if (p.Side == types.PosLong && m.Mark.LessThanOrEqual(liq)) ||
			(p.Side == types.PosShort && m.Mark.GreaterThanOrEqual(liq)) {
			_, _ = e.ClosePosition(p.UserID, p.ID)
		}
	}
}

func fillPnL(p *types.Position, mark decimal.Decimal) {
	qty := dec(p.Qty)
	entry := dec(p.EntryPrice)
	margin := dec(p.Margin)
	var pnl decimal.Decimal
	if p.Side == types.PosLong {
		pnl = mark.Sub(entry).Mul(qty)
	} else {
		pnl = entry.Sub(mark).Mul(qty)
	}
	p.Unrealized = pnl.StringFixed(8)
	p.MarkPrice = mark.String()
	if !margin.IsZero() {
		p.ROE = pnl.Div(margin).StringFixed(6)
	}
	mm := decimal.RequireFromString("0.005")
	if p.Side == types.PosLong {
		p.LiqPrice = entry.Sub(margin.Div(qty)).Add(entry.Mul(mm)).StringFixed(8)
	} else {
		p.LiqPrice = entry.Add(margin.Div(qty)).Sub(entry.Mul(mm)).StringFixed(8)
	}
}

type PlaceReq struct {
	Symbol     string          `json:"symbol"`
	Side       types.Side      `json:"side"`
	PosSide    types.PosSide   `json:"pos_side"`
	Type       types.OrderType `json:"type"`
	Price      string          `json:"price"`
	Qty        string          `json:"qty"`
	Leverage   int             `json:"leverage"`
	ReduceOnly bool            `json:"reduce_only"`
}

func dec(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
func f(v float64) string { return decimal.NewFromFloat(v).String() }
func maxDec(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}
func minDec(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

type errf string

func (e errf) Error() string { return string(e) }
