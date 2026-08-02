package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/types"
)

type Server struct {
	ledger *account.Ledger
	engine *engine.Engine
	mux    *http.ServeMux
}

func New(ledger *account.Ledger, eng *engine.Engine) *Server {
	s := &Server{ledger: ledger, engine: eng, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/book", s.book)
	s.mux.HandleFunc("GET /api/trades", s.trades)
	s.mux.HandleFunc("GET /api/balances", s.balances)
	s.mux.HandleFunc("POST /api/deposit", s.deposit)
	s.mux.HandleFunc("POST /api/orders", s.placeOrder)
	s.mux.HandleFunc("GET /api/orders", s.openOrders)
	s.mux.HandleFunc("DELETE /api/orders/{id}", s.cancelOrder)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Id")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "symbol": s.engine.Symbol()})
}

func (s *Server) book(w http.ResponseWriter, r *http.Request) {
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
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
	var body struct {
		Asset  string  `json:"asset"`
		Amount float64 `json:"amount"`
	}
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
	writeJSON(w, http.StatusCreated, map[string]any{
		"order":  order,
		"trades": trades,
	})
}

func (s *Server) openOrders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.OpenOrders(userID(r)))
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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

func userID(r *http.Request) string {
	u := r.Header.Get("X-User-Id")
	if u == "" {
		u = r.URL.Query().Get("user_id")
	}
	if u == "" {
		u = "alice"
	}
	return u
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
