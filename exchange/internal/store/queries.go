package store

import (
	"embed"
	"strings"
)

//go:embed queries/*.sql
var queryFS embed.FS

// 业务 SQL 从 queries/*.sql 加载（与 migrations 分开：migrations=改表结构，queries=日常增删改查）
var (
	sqlUpsertBalance  string
	sqlLoadBalances   string
	sqlHasAnyBalance  string
	sqlAppendLedger   string
	sqlListLedger     string
	sqlUpsertOrder    string
	sqlLoadOrders     string
	sqlInsertTrade    string
	sqlLoadTrades     string
)

func init() {
	sqlUpsertBalance = mustSQL("queries/upsert_balance.sql")
	sqlLoadBalances = mustSQL("queries/load_balances.sql")
	sqlHasAnyBalance = mustSQL("queries/has_any_balance.sql")
	sqlAppendLedger = mustSQL("queries/append_ledger.sql")
	sqlListLedger = mustSQL("queries/list_ledger.sql")
	sqlUpsertOrder = mustSQL("queries/upsert_order.sql")
	sqlLoadOrders = mustSQL("queries/load_orders.sql")
	sqlInsertTrade = mustSQL("queries/insert_trade.sql")
	sqlLoadTrades = mustSQL("queries/load_trades.sql")
}

func mustSQL(name string) string {
	b, err := queryFS.ReadFile(name)
	if err != nil {
		panic("load sql " + name + ": " + err.Error())
	}
	// 去掉纯注释行，避免个别驱动对前导注释敏感；保留 SQL 本体
	lines := strings.Split(string(b), "\n")
	var keep []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "--") {
			continue
		}
		keep = append(keep, line)
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}
