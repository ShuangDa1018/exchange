package ranking

import "testing"

func TestEquity(t *testing.T) {
	if got := Equity(10000, 1, 100); got != 10100 {
		t.Fatalf("got %v", got)
	}
	if got := Equity(10000, 1, 0); got != 10100 {
		t.Fatalf("default price: %v", got)
	}
}

func TestNotional(t *testing.T) {
	if got := notional(nil); got != 0 {
		t.Fatalf("nil: %v", got)
	}
}
