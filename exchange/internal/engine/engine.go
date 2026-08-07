// Package engine：撮合引擎（交易所最核心的逻辑）
// 职责：接单 → 冻结资金 → 和对手盘撮合 → 成交结算 / 挂到订单簿
//
// 标记约定（全文可搜）：
//   [MEM]       只读写内存订单簿（e.orders / bids / asks / trades）
//   [DB:READ]   从 Postgres 读（仅启动 Restore）
//   [DB:WRITE]  写入 Postgres（下单/撤单/成交后落盘）
//
// 注意：运行中改 TablePlus 的 orders 表不会影响 [MEM]；
//       要生效需重启进程（再走 Restore），或走 API 下单/撤单。
package engine

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/mq"
	"github.com/demo/exchange/internal/store"
	"github.com/demo/exchange/internal/types"
)

// Engine 现货撮合引擎
// 规则：价格优先，同价则时间优先（先来的先成交）
type Engine struct {
	mu      sync.Mutex
	symbol  string
	base    string
	quote   string
	ledger  *account.Ledger
	db      *store.DB              // [DB] 落盘/恢复；热路径不查库
	pub     mq.TradePublisher      // [Kafka] 成交事件；可为 Noop
	bids    []*types.Order         // [MEM] 买盘
	asks    []*types.Order         // [MEM] 卖盘
	orders  map[int64]*types.Order // [MEM] 全部订单索引
	trades  []*types.Trade         // [MEM] 成交历史
	nextOID int64
	nextTID int64
}

// New 创建引擎；db 非空时 [DB:READ] 恢复挂单/成交；pub 可为 nil（用 Noop）
func New(symbol, base, quote string, ledger *account.Ledger, db *store.DB, pub mq.TradePublisher) *Engine {
	if pub == nil {
		pub = mq.NoopPublisher{}
	}
	e := &Engine{
		symbol: symbol,
		base:   base,
		quote:  quote,
		ledger: ledger,
		db:     db,
		pub:    pub,
		orders: make(map[int64]*types.Order),
		trades: make([]*types.Trade, 0),
	}
	if db != nil {
		if err := e.Restore(); err != nil { // [DB:READ]
			log.Printf("恢复订单簿失败: %v", err)
		}
	}
	return e
}

func (e *Engine) Symbol() string { return e.symbol }

// Restore [DB:READ]→[MEM] 启动时：库 → 内存（之后查询不再读 orders 表）
func (e *Engine) Restore() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	orders, err := e.db.LoadOrders(ctx) // [DB:READ]
	if err != nil {
		return err
	}
	for _, o := range orders {
		e.orders[o.ID] = o // [MEM] 写入内存
		if o.ID > e.nextOID {
			e.nextOID = o.ID
		}
		if (o.Status == types.StatusOpen || o.Status == types.StatusPartial) && o.Remaining() > 0 {
			e.rest(o) // [MEM] 挂到 bids/asks
		}
	}
	trades, err := e.db.LoadTrades(ctx) // [DB:READ]
	if err != nil {
		return err
	}
	e.trades = trades // [MEM]
	for _, t := range trades {
		if t.ID > e.nextTID {
			e.nextTID = t.ID
		}
	}
	log.Printf("已从 Postgres 恢复: 订单 %d 笔, 成交 %d 笔, 买盘 %d / 卖盘 %d",
		len(e.orders), len(e.trades), len(e.bids), len(e.asks))
	return nil
}

// saveOrder [DB:WRITE] 订单落盘
func (e *Engine) saveOrder(o *types.Order) {
	if e.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := e.db.UpsertOrder(ctx, o); err != nil { // [DB:WRITE]
		log.Printf("落盘订单失败 #%d: %v", o.ID, err)
	}
}

// saveTrade [DB:WRITE] 成交落盘
func (e *Engine) saveTrade(t *types.Trade) {
	if e.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := e.db.InsertTrade(ctx, t); err != nil { // [DB:WRITE]
		log.Printf("落盘成交失败 #%d: %v", t.ID, err)
	}
}

// Place [MEM] 撮合 + 挂簿；结束后 [DB:WRITE] 落盘
// 多返回值：(订单, 本次产生的成交列表, 错误)
func (e *Engine) Place(userID string, side types.Side, price, qty float64) (*types.Order, []*types.Trade, error) {
	if price <= 0 || qty <= 0 {
		return nil, nil, fmt.Errorf("价格和数量必须大于 0")
	}
	if side != types.SideBuy && side != types.SideSell {
		return nil, nil, fmt.Errorf("side 只能是 buy 或 sell")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 买单冻 USDT（price * qty）；卖单冻 BTC（qty）
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
	log.Printf("下单 #%d: %s %s %f %f", e.nextOID, userID, side, price, qty)
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
	e.orders[order.ID] = order // [MEM]

	trades := e.match(order) // [MEM] 撮合
	if order.Remaining() > 0 && order.Status != types.StatusFilled {
		e.rest(order) // [MEM] 剩余挂簿
	}
	e.saveOrder(order) // [DB:WRITE] 本单落盘
	for _, t := range trades {
		e.saveTrade(t) // [DB:WRITE] 成交落盘
	}
	return cloneOrder(order), cloneTrades(trades), nil
}

// Cancel [MEM] 改内存状态；再 [DB:WRITE] 落盘
func (e *Engine) Cancel(userID string, orderID int64) (*types.Order, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	order, ok := e.orders[orderID] // [MEM] 查的是内存，不是 DB
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

	order.Status = types.StatusCancelled // [MEM]
	e.removeFromBook(order)              // [MEM]
	e.saveOrder(order)                   // [DB:WRITE]
	return cloneOrder(order), nil
}

// GetOrder [MEM] 只查内存
func (e *Engine) GetOrder(id int64) (*types.Order, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	o, ok := e.orders[id]
	if !ok {
		return nil, false
	}
	return cloneOrder(o), true
}

// OpenOrders [MEM] 只扫内存 e.orders（改 DB 不会反映到这里）
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

// RecentTrades [MEM] 只读内存 e.trades
func (e *Engine) RecentTrades(limit int) []*types.Trade {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit <= 0 || limit > len(e.trades) {
		limit = len(e.trades)
	}
	start := len(e.trades) - limit
	return cloneTrades(e.trades[start:])
}

// BookLevel 订单簿的一档：同一价格汇总后的量
type BookLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// OrderBookSnapshot 盘口快照（前端订单簿直接渲染这个）
type OrderBookSnapshot struct {
	Symbol string      `json:"symbol"`
	Bids   []BookLevel `json:"bids"` // 买盘：价格从高到低
	Asks   []BookLevel `json:"asks"` // 卖盘：价格从低到高
}

// Snapshot [MEM] 盘口快照，只聚合内存 bids/asks
func (e *Engine) Snapshot(depth int) OrderBookSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	if depth <= 0 {
		depth = 10
	}
	return OrderBookSnapshot{
		Symbol: e.symbol,
		Bids:   aggregate(e.bids, depth), // [MEM]
		Asks:   aggregate(e.asks, depth), // [MEM]
	}
}

// match [MEM] 核心撮合：只改内存订单；结算走 ledger（会落库余额）
func (e *Engine) match(taker *types.Order) []*types.Trade {
	var trades []*types.Trade // 零值 nil slice，可以 append

	for taker.Remaining() > 0 {
		var maker *types.Order
		if taker.Side == types.SideBuy {
			// 没有卖单，或最便宜的卖价仍高于我的买价 → 撮合不了
			if len(e.asks) == 0 || e.asks[0].Price > taker.Price {
				break
			}
			maker = e.asks[0] // 最优卖单（最低价）
		} else {
			if len(e.bids) == 0 || e.bids[0].Price < taker.Price {
				break
			}
			maker = e.bids[0] // 最优买单（最高价）
		}

		// 成交价取 maker 价（挂单方价格）——交易所常见规则
		tradePrice := maker.Price
		qty := min(taker.Remaining(), maker.Remaining())
		cost := tradePrice * qty

		taker.Filled += qty
		maker.Filled += qty
		updateStatus(taker)
		updateStatus(maker)

		// 两边账本结算
		if taker.Side == types.SideBuy {
			e.ledger.SettleBuy(taker.UserID, e.base, e.quote, qty, cost)
			e.ledger.SettleSell(maker.UserID, e.base, e.quote, qty, cost)
		} else {
			e.ledger.SettleSell(taker.UserID, e.base, e.quote, qty, cost)
			e.ledger.SettleBuy(maker.UserID, e.base, e.quote, qty, cost)
		}

		// 买单按更高限价冻结，实际成交更便宜 → 退回差价
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
		e.saveOrder(maker) // [DB:WRITE] maker 状态可能变 partial/filled
		// [Kafka] 异步事件：让行情/通知等下游消费（失败只打日志）
		go func(t *types.Trade) {
			_ = e.pub.PublishTrade(context.Background(), t)
		}(tr)

		if maker.Remaining() == 0 {
			e.removeFromBook(maker)
		}
	}
	return trades
}

// rest 未完全成交的单，挂到订单簿等待
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

// 买盘排序：价格高优先；同价 ID 小（更早）优先
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

// 卖盘排序：价格低优先；同价 ID 小优先
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

// filterOut 按 id 过滤 ≈ arr.filter(o => o.id !== id)
func filterOut(book []*types.Order, id int64) []*types.Order {
	out := book[:0] // 复用底层数组的小技巧，效果类似新建空数组再 push
	for _, o := range book {
		if o.ID != id {
			out = append(out, o)
		}
	}
	return out
}

// aggregate 把同一价格的挂单数量合并成一档（前端订单簿展示用）
func aggregate(book []*types.Order, depth int) []BookLevel {
	levels := make([]BookLevel, 0, depth)
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
		levels = append(levels, BookLevel{Price: o.Price, Quantity: o.Remaining()})
	}
	return levels
}

// switch 无表达式写法 ≈ if / else if 链
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

// cloneOrder 浅拷贝结构体，避免返回内部指针被外部修改
func cloneOrder(o *types.Order) *types.Order {
	c := *o   // 拷贝一份值
	return &c // 再取地址返回
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
