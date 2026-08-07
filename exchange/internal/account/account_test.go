package account_test

import (
	"testing"

	"github.com/demo/exchange/internal/account"
)

func TestLockUnlock(t *testing.T) {
	l := account.NewLedger(nil, nil)
	l.EnsureUser("alice", map[string]float64{"USDT": 1000})

	if err := l.Lock("alice", "USDT", 300); err != nil {
		t.Fatal(err)
	}
	b := l.GetBalances("alice")["USDT"]
	if b.Available != 700 || b.Locked != 300 {
		t.Fatalf("after lock: %+v", b)
	}

	l.Unlock("alice", "USDT", 100)
	b = l.GetBalances("alice")["USDT"]
	if b.Available != 800 || b.Locked != 200 {
		t.Fatalf("after unlock: %+v", b)
	}
}

func TestLockInsufficient(t *testing.T) {
	l := account.NewLedger(nil, nil)
	l.EnsureUser("alice", map[string]float64{"USDT": 10})
	if err := l.Lock("alice", "USDT", 50); err == nil {
		t.Fatal("expected insufficient balance error")
	}
}
