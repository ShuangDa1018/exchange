package engine_test

import (
	"testing"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/types"
)

func TestMatchBuyTakesSell(t *testing.T) {
	ledger := account.NewLedger(nil, nil)
	ledger.EnsureUser("alice", map[string]float64{"USDT": 100000, "BTC": 0})
	ledger.EnsureUser("bob", map[string]float64{"USDT": 0, "BTC": 2})

	eng := engine.New("BTC-USDT", "BTC", "USDT", ledger, nil, nil)

	// bob 挂卖 1 BTC @ 50000
	if _, _, err := eng.Place("bob", types.SideSell, 50000, 1); err != nil {
		t.Fatal(err)
	}
	// alice 买 1 BTC @ 51000 → 应按 maker 价 50000 成交
	order, trades, err := eng.Place("alice", types.SideBuy, 51000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != types.StatusFilled {
		t.Fatalf("want filled, got %s", order.Status)
	}
	if len(trades) != 1 || trades[0].Price != 50000 || trades[0].Quantity != 1 {
		t.Fatalf("unexpected trades: %+v", trades)
	}

	aliceBTC := ledger.GetBalances("alice")["BTC"].Available
	if aliceBTC != 1 {
		t.Fatalf("alice BTC=%v want 1", aliceBTC)
	}
}
