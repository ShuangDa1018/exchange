package types

import "time"

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type PosSide string

const (
	PosLong  PosSide = "long"
	PosShort PosSide = "short"
)

type OrderType string

const (
	OrderMarket OrderType = "market"
	OrderLimit  OrderType = "limit"
)

type OrderStatus string

const (
	StatusNew       OrderStatus = "new"
	StatusFilled    OrderStatus = "filled"
	StatusPartial   OrderStatus = "partial"
	StatusCancelled OrderStatus = "cancelled"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	WalletEVM    string    `json:"wallet_evm"`
	WalletSOL    string    `json:"wallet_sol"`
	ReferralCode string    `json:"referral_code"`
	InvitedBy    string    `json:"invited_by,omitempty"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
}

type Balance struct {
	Asset     string `json:"asset"`
	Available string `json:"available"`
	Locked    string `json:"locked"`
}

type Market struct {
	Symbol        string `json:"symbol"`
	Base          string `json:"base"`
	Quote         string `json:"quote"`
	Type          string `json:"type"`
	TickSize      string `json:"tick_size"`
	LotSize       string `json:"lot_size"`
	MaxLeverage   int    `json:"max_leverage"`
	TakerFee      string `json:"taker_fee"`
	MakerFee      string `json:"maker_fee"`
	MaintMargin   string `json:"maint_margin"`
	FundingRate   string `json:"funding_rate"`
	Price         string `json:"price"`
	MarkPrice     string `json:"mark_price"`
	IndexPrice    string `json:"index_price"`
	Change24h     string `json:"change_24h"`
	High24h       string `json:"high_24h"`
	Low24h        string `json:"low_24h"`
	Volume24h     string `json:"volume_24h"`
	OpenInterest  string `json:"open_interest"`
}

type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Symbol    string      `json:"symbol"`
	Side      Side        `json:"side"`
	PosSide   PosSide     `json:"pos_side"`
	Type      OrderType   `json:"type"`
	Price     string      `json:"price"`
	Qty       string      `json:"qty"`
	FilledQty string      `json:"filled_qty"`
	Leverage  int         `json:"leverage"`
	Status    OrderStatus `json:"status"`
	Fee       string      `json:"fee"`
	ReduceOnly bool       `json:"reduce_only"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Position struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Symbol       string    `json:"symbol"`
	Side         PosSide   `json:"side"`
	Qty          string    `json:"qty"`
	EntryPrice   string    `json:"entry_price"`
	MarkPrice    string    `json:"mark_price"`
	Leverage     int       `json:"leverage"`
	Margin       string    `json:"margin"`
	Unrealized   string    `json:"unrealized_pnl"`
	LiqPrice     string    `json:"liq_price"`
	ROE          string    `json:"roe"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Trade struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     string    `json:"price"`
	Qty       string    `json:"qty"`
	Side      Side      `json:"side"`
	BuyerID   string    `json:"buyer_id,omitempty"`
	SellerID  string    `json:"seller_id,omitempty"`
	Fee       string    `json:"fee"`
	CreatedAt time.Time `json:"created_at"`
}

type Candle struct {
	Time   int64  `json:"time"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

type BookLevel struct {
	Price string `json:"price"`
	Qty   string `json:"qty"`
}

type OrderBook struct {
	Symbol string      `json:"symbol"`
	Bids   []BookLevel `json:"bids"`
	Asks   []BookLevel `json:"asks"`
	Ts     int64       `json:"ts"`
}

type Ledger struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Asset     string    `json:"asset"`
	Amount    string    `json:"amount"`
	Kind      string    `json:"kind"` // deposit|withdraw|fee|pnl|rebate|points
	Ref       string    `json:"ref"`
	CreatedAt time.Time `json:"created_at"`
}

type Points struct {
	UserID    string `json:"user_id"`
	Balance   string `json:"balance"`
	VolumeUSD string `json:"volume_usd"`
}

type ReferralStats struct {
	Code       string `json:"code"`
	Invitees   int    `json:"invitees"`
	RebateUSDT string `json:"rebate_usdt"`
	Points     string `json:"points"`
}
