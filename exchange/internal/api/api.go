// Package api：HTTP 接口层（≈ Express / Next.js Route Handlers）
// 只负责：解析请求 → 调 engine/account → 返回 JSON
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/types"
)

// Server 把账本 + 撮合引擎挂到路由上
type Server struct {
	ledger *account.Ledger
	engine *engine.Engine
	mux    *http.ServeMux // 路由器 ≈ express.Router()
}

func New(ledger *account.Ledger, eng *engine.Engine) *Server {
	s := &Server{ledger: ledger, engine: eng, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 把自己当成 http.Handler 返回（因为实现了 ServeHTTP）
func (s *Server) Handler() http.Handler { return s }

// routes 注册路由，写法像：METHOD + path
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/book", s.book)             // 订单簿
	s.mux.HandleFunc("GET /api/trades", s.trades)         // 最近成交
	s.mux.HandleFunc("GET /api/balances", s.balances)     // 余额
	s.mux.HandleFunc("POST /api/deposit", s.deposit)      // 充值（演示）
	s.mux.HandleFunc("POST /api/orders", s.placeOrder)    // 下单
	s.mux.HandleFunc("GET /api/orders", s.openOrders)     // 我的挂单
	s.mux.HandleFunc("DELETE /api/orders/{id}", s.cancelOrder) // 撤单
}

// ServeHTTP ≈ middleware + router
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Id")
	if r.Method == http.MethodOptions { // 浏览器预检请求
		w.WriteHeader(http.StatusNoContent) // 204
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	// _ 表示这个参数用不到，但函数签名必须留着
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "symbol": s.engine.Symbol()})
}

// GET /api/book?depth=8
func (s *Server) book(w http.ResponseWriter, r *http.Request) {
	// r.URL.Query().Get ≈ new URLSearchParams(location.search).get
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth")) // Atoi = string → int；_ 忽略错误
	writeJSON(w, http.StatusOK, s.engine.Snapshot(depth))
}

func (s *Server) trades(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	writeJSON(w, http.StatusOK, s.engine.RecentTrades(limit))
}

func (s *Server) balances(w http.ResponseWriter, r *http.Request) {
	user := userID(r)
	writeJSON(w, http.StatusOK, s.ledger.GetBalances(user))
}

func (s *Server) deposit(w http.ResponseWriter, r *http.Request) {
	user := userID(r)
	// 匿名 struct：只在这里用一次的请求体类型
	// ≈ const body = await req.json() as { asset: string; amount: number }
	var body struct {
		Asset  string  `json:"asset"`
		Amount float64 `json:"amount"`
	}
	// Decode(&body)：必须传指针，才能把 JSON 写进 body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Asset == "" || body.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "需要 asset 和 amount"})
		return
	}
	if err := s.ledger.Deposit(user, body.Asset, body.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.ledger.GetBalances(user))
}

func (s *Server) placeOrder(w http.ResponseWriter, r *http.Request) {
	user := userID(r)
	var body struct {
		Side     types.Side `json:"side"`
		Price    float64    `json:"price"`
		Quantity float64    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效 JSON"})
		return
	}
	order, trades, err := s.engine.Place(user, body.Side, body.Price, body.Quantity)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// map[string]any ≈ Record<string, unknown>，any 是任意类型
	writeJSON(w, http.StatusCreated, map[string]any{
		"order":  order,
		"trades": trades,
	})
}

func (s *Server) openOrders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.OpenOrders(userID(r)))
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	// PathValue("id") ≈ req.params.id
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64) // 10 进制，64 位整数
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效订单 ID"})
		return
	}
	order, err := s.engine.Cancel(userID(r), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// userID 从请求头取当前用户（演示用，正式项目会用登录 token）
// 前端对应：fetch(url, { headers: { 'X-User-Id': 'alice' } })
func userID(r *http.Request) string {
	u := r.Header.Get("X-User-Id")
	if u == "" {
		u = r.URL.Query().Get("user_id")
	}
	if u == "" {
		u = "alice" // 默认用户
	}
	return u
}

// writeJSON 统一写 JSON 响应 ≈ res.status(code).json(data)
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status) // 先设 status，再写 body
	_ = json.NewEncoder(w).Encode(v)
}
