package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/api"
	"github.com/demo/exchange/internal/engine"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	ledger := account.NewLedger()
	// 预置两个用户，方便本地对敲演示
	ledger.EnsureUser("alice", map[string]float64{"USDT": 10000, "BTC": 1})
	ledger.EnsureUser("bob", map[string]float64{"USDT": 10000, "BTC": 1})

	eng := engine.New("BTC-USDT", "BTC", "USDT", ledger)
	apiServer := api.New(ledger, eng)

	web, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(web))

	// 手动分流，避免 Go 1.22+ ServeMux 中 "/api/" 与 "GET /" 冲突
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := ":8080"
	log.Printf("交易所 Demo 已启动: http://localhost%s", addr)
	log.Printf("交易对: BTC-USDT | 用户: alice / bob（请求头 X-User-Id）")
	log.Printf("  GET  /api/book")
	log.Printf("  GET  /api/trades")
	log.Printf("  GET  /api/balances")
	log.Printf("  POST /api/orders")
	log.Printf("  DELETE /api/orders/{id}")

	if err := http.ListenAndServe(addr, withCORS(root)); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
