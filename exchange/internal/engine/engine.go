package engine

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/types"
)

// Engine 现货撮合引擎（价格优先 + 时间优先）
type Engine struct {
	mu       sync.Mutex
	symbol   string
	base     string // BTC
	quote    string // USDT
	ledger   *account.Ledger
	bids     []*types.Order // 买盘：价格高优先，同价时间早优先
	asks     []*types.Order // 卖盘：价格低优先，同价时间早优先
	orders   map[int64]*types.Order
	trades   []*types.Trade
	nextOID  int64
	nextTID  int64
}

func New(symbol, base, quote string, ledger *account.Ledger) *Engine {
	return &Engine{
		symbol: symbol,
		base:   base,
		quote:  quote,
		ledger: ledger,
		orders: make(map[int64]*types.Order),
		trades: make([]*types.Trade, 0),
	}
}

func (e *Engine) Symbol() string { return e.symbol }

// Place 挂限价单并尝试撮合
func (e *Engine) Place(userID string, side types.Side, price, qty float64) (*types.Order, []*types.Trade, error) {
	if price <= 0 || qty <= 0 {
		return nil, nil, fmt.Errorf("价格和数量必须大于 0")
	}
	if side != types.SideBuy && side != types.SideSell {
		return nil, nil, fmt.Errorf("side 只能是 buy 或 sell")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 冻结资金：买单冻 USDT，卖单冻 BTC
	if side == types.SideBuy {
		if err := e.ledger.Lock(userID, e.quote, price*qty); err != nil {
			return nil, nil, err
		}
	} else {
		if err := e.ledger.Lock(userID, e.base, qty); err != nil {
			return nil, nil, err
		}
	}

	e.nextOID++
	order := &types.Order{
		ID:        e.nextOID,
		UserID:    userID,
		Symbol:    e.symbol,
		Side:      side,
		Price:     price,
		Quantity:  qty,
		Status:    types.StatusOpen,
		CreatedAt: time.Now().UTC(),
	}
	e.orders[order.ID] = order

	trades := e.match(order)
	if order.Remaining() > 0 && order.Status != types.StatusFilled {
		e.rest(order)
	}
	return cloneOrder(order), cloneTrades(trades), nil
}

// Cancel 撤单并解冻剩余
func (e *Engine) Cancel(userID string, orderID int64) (*types.Order, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	order, ok := e.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("订单不存在")
	}
	if order.UserID != userID {
		return nil, fmt.Errorf("无权撤销该订单")
	}
	if order.Status == types.StatusFilled || order.Status == types.StatusCancelled {
		return nil, fmt.Errorf("订单已结束，无法撤销")
	}

	remain := order.Remaining()
	if order.Side == types.SideBuy {
		e.ledger.Unlock(userID, e.quote, order.Price*remain)
	} else {
		e.ledger.Unlock(userID, e.base, remain)
	}

	order.Status = types.StatusCancelled
	e.removeFromBook(order)
	return cloneOrder(order), nil
}

func (e *Engine) GetOrder(id int64) (*types.Order, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	o, ok := e.orders[id]
	if !ok {
		return nil, false
	}
	return cloneOrder(o), true
}

func (e *Engine) OpenOrders(userID string) []*types.Order {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*types.Order, 0)
	for _, o := range e.orders {
		if o.UserID == userID && (o.Status == types.StatusOpen || o.Status == types.StatusPartial) {
			out = append(out, cloneOrder(o))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func (e *Engine) RecentTrades(limit int) []*types.Trade {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit <= 0 || limit > len(e.trades) {
		limit = len(e.trades)
	}
	start := len(e.trades) - limit
	return cloneTrades(e.trades[start:])
}

func (e *Engine) Snapshot(depth int) types.OrderBookSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	if depth <= 0 {
		depth = 10
	}
	return types.OrderBookSnapshot{
		Symbol: e.symbol,
		Bids:   aggregate(e.bids, depth),
		Asks:   aggregate(e.asks, depth),
	}
}

// match 吃单：新单与对手盘撮合
func (e *Engine) match(taker *types.Order) []*types.Trade {
	var trades []*types.Trade

	for taker.Remaining() > 0 {
		var maker *types.Order
		if taker.Side == types.SideBuy {
			if len(e.asks) == 0 || e.asks[0].Price > taker.Price {
				break // 买不起最优卖价
			}
			maker = e.asks[0]
		} else {
			if len(e.bids) == 0 || e.bids[0].Price < taker.Price {
				break // 卖不掉最优买价
			}
			maker = e.bids[0]
		}

		// 成交价取挂单方（maker）价格
		tradePrice := maker.Price
		qty := min(taker.Remaining(), maker.Remaining())
		cost := tradePrice * qty

		taker.Filled += qty
		maker.Filled += qty
		updateStatus(taker)
		updateStatus(maker)

		if taker.Side == types.SideBuy {
			e.ledger.SettleBuy(taker.UserID, e.base, e.quote, qty, cost)
			e.ledger.SettleSell(maker.UserID, e.base, e.quote, qty, cost)
		} else {
			e.ledger.SettleSell(taker.UserID, e.base, e.quote, qty, cost)
			e.ledger.SettleBuy(maker.UserID, e.base, e.quote, qty, cost)
		}

		// 买单冻结的是 taker.Price，实际成交可能更低，退回差价
		if taker.Side == types.SideBuy && taker.Price > tradePrice {
			e.ledger.Unlock(taker.UserID, e.quote, (taker.Price-tradePrice)*qty)
		}

		e.nextTID++
		tr := &types.Trade{
			ID:        e.nextTID,
			Symbol:    e.symbol,
			Price:     tradePrice,
			Quantity:  qty,
			CreatedAt: time.Now().UTC(),
		}
		if taker.Side == types.SideBuy {
			tr.BuyOrder, tr.SellOrder = taker.ID, maker.ID
			tr.BuyerID, tr.SellerID = taker.UserID, maker.UserID
		} else {
			tr.BuyOrder, tr.SellOrder = maker.ID, taker.ID
			tr.BuyerID, tr.SellerID = maker.UserID, taker.UserID
		}
		e.trades = append(e.trades, tr)
		trades = append(trades, tr)

		if maker.Remaining() == 0 {
			e.removeFromBook(maker)
		}
	}
	return trades
}

func (e *Engine) rest(order *types.Order) {
	if order.Side == types.SideBuy {
		e.bids = insertBid(e.bids, order)
	} else {
		e.asks = insertAsk(e.asks, order)
	}
}

func (e *Engine) removeFromBook(order *types.Order) {
	if order.Side == types.SideBuy {
		e.bids = filterOut(e.bids, order.ID)
	} else {
		e.asks = filterOut(e.asks, order.ID)
	}
}

func insertBid(book []*types.Order, o *types.Order) []*types.Order {
	book = append(book, o)
	sort.SliceStable(book, func(i, j int) bool {
		if book[i].Price == book[j].Price {
			return book[i].ID < book[j].ID
		}
		return book[i].Price > book[j].Price
	})
	return book
}

func insertAsk(book []*types.Order, o *types.Order) []*types.Order {
	book = append(book, o)
	sort.SliceStable(book, func(i, j int) bool {
		if book[i].Price == book[j].Price {
			return book[i].ID < book[j].ID
		}
		return book[i].Price < book[j].Price
	})
	return book
}

func filterOut(book []*types.Order, id int64) []*types.Order {
	out := book[:0]
	for _, o := range book {
		if o.ID != id {
			out = append(out, o)
		}
	}
	return out
}

func aggregate(book []*types.Order, depth int) []types.BookLevel {
	levels := make([]types.BookLevel, 0, depth)
	for _, o := range book {
		if o.Remaining() <= 0 {
			continue
		}
		n := len(levels)
		if n > 0 && levels[n-1].Price == o.Price {
			levels[n-1].Quantity += o.Remaining()
			continue
		}
		if len(levels) >= depth {
			break
		}
		levels = append(levels, types.BookLevel{Price: o.Price, Quantity: o.Remaining()})
	}
	return levels
}

func updateStatus(o *types.Order) {
	switch {
	case o.Filled <= 0:
		o.Status = types.StatusOpen
	case o.Filled < o.Quantity:
		o.Status = types.StatusPartial
	default:
		o.Status = types.StatusFilled
	}
}

func cloneOrder(o *types.Order) *types.Order {
	c := *o
	return &c
}

func cloneTrades(in []*types.Trade) []*types.Trade {
	out := make([]*types.Trade, len(in))
	for i, t := range in {
		c := *t
		out[i] = &c
	}
	return out
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
