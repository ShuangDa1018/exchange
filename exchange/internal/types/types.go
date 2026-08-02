package types

import "time"

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type OrderStatus string

const (
	StatusOpen      OrderStatus = "open"
	StatusPartial   OrderStatus = "partial"
	StatusFilled    OrderStatus = "filled"
	StatusCancelled OrderStatus = "cancelled"
)

// Order 限价单
type Order struct {
	ID        int64       `json:"id"`
	UserID    string      `json:"user_id"`
	Symbol    string      `json:"symbol"` // 如 BTC-USDT
	Side      Side        `json:"side"`
	Price     float64     `json:"price"`
	Quantity  float64     `json:"quantity"` // 原始数量
	Filled    float64     `json:"filled"`   // 已成交数量
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

func (o *Order) Remaining() float64 {
	return o.Quantity - o.Filled
}

// Trade 成交记录
type Trade struct {
	ID        int64     `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	BuyOrder  int64     `json:"buy_order_id"`
	SellOrder int64     `json:"sell_order_id"`
	BuyerID   string    `json:"buyer_id"`
	SellerID  string    `json:"seller_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BookLevel 订单簿一档
type BookLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// OrderBookSnapshot 盘口快照
type OrderBookSnapshot struct {
	Symbol string      `json:"symbol"`
	Bids   []BookLevel `json:"bids"` // 买盘，价格从高到低
	Asks   []BookLevel `json:"asks"` // 卖盘，价格从低到高
}
