// Package types：共享的数据类型（≈ 前端的 types.ts / interfaces）
package types

import "time"

// Side 用自定义 string 类型，限制只能是 buy/sell
// ≈ type Side = 'buy' | 'sell'
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// OrderStatus ≈ type OrderStatus = 'open' | 'partial' | 'filled' | 'cancelled'
type OrderStatus string

const (
	StatusOpen      OrderStatus = "open"      // 挂着，还没成交
	StatusPartial   OrderStatus = "partial"   // 部分成交
	StatusFilled    OrderStatus = "filled"    // 全部成交
	StatusCancelled OrderStatus = "cancelled" // 已撤单
)

// Order 限价单 ≈ interface Order { ... }
// 字段大写开头才能被 JSON 导出；`json:"xxx"` 决定前端看到的字段名
type Order struct {
	ID        int64       `json:"id"`
	UserID    string      `json:"user_id"`
	Symbol    string      `json:"symbol"`   // 交易对，如 BTC-USDT
	Side      Side        `json:"side"`     // buy / sell
	Price     float64     `json:"price"`    // 限价
	Quantity  float64     `json:"quantity"` // 原始下单数量
	Filled    float64     `json:"filled"`   // 已成交数量
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

// Remaining 剩余未成交量
// (o *Order) 是方法接收者 ≈ class Order { remaining() { ... } }
func (o *Order) Remaining() float64 {
	return o.Quantity - o.Filled
}

// Trade 一笔成交记录（买卖双方配对成功后产生）
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
