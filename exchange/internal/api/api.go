// Package api：HTTP 接口层（Gin）
// 只负责：解析请求 → 调 engine/account → 返回 JSON
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/cache"
	"github.com/demo/exchange/internal/engine"
	"github.com/demo/exchange/internal/types"
	"github.com/gin-gonic/gin"
)

// Server 把账本 + 撮合引擎挂到 Gin 路由上
type Server struct {
	ledger *account.Ledger
	engine *engine.Engine
	rdb    *cache.Redis
	router *gin.Engine
}

func New(ledger *account.Ledger, eng *engine.Engine, rdb *cache.Redis) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(rateLimitMiddleware(rdb)) // [Redis:WRITE] 限流

	s := &Server{ledger: ledger, engine: eng, rdb: rdb, router: r}
	s.routes()
	return s
}

// Engine 返回 Gin 引擎，供 main 挂静态资源并 Listen
func (s *Server) Engine() *gin.Engine { return s.router }

func (s *Server) routes() {
	api := s.router.Group("/api")
	{
		api.GET("/health", s.health)
		api.GET("/book", s.book)                 // 订单簿
		api.GET("/trades", s.trades)             // 最近成交
		api.GET("/balances", s.balances)         // [Redis:READ]
		api.GET("/ledger", s.ledgerEntries)      // [DB:READ]
		api.POST("/deposit", s.deposit)          // [DB:WRITE]
		api.POST("/orders", s.placeOrder)        // [DB:WRITE]
		api.GET("/orders", s.openOrders)         // 内存挂单
		api.DELETE("/orders/:id", s.cancelOrder) // [DB:WRITE]
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "symbol": s.engine.Symbol()})
}

// GET /api/book?depth=8
func (s *Server) book(c *gin.Context) {
	depth, _ := strconv.Atoi(c.Query("depth"))
	c.JSON(http.StatusOK, s.engine.Snapshot(depth))
}

func (s *Server) trades(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 20
	}
	c.JSON(http.StatusOK, s.engine.RecentTrades(limit))
}

func (s *Server) balances(c *gin.Context) {
	c.JSON(http.StatusOK, s.ledger.GetBalances(userID(c))) // [Redis:READ]
}

// GET /api/ledger [DB:READ]
func (s *Server) ledgerEntries(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	entries, err := s.ledger.ListLedger(userID(c), limit) // [DB:READ]
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (s *Server) deposit(c *gin.Context) {
	user := userID(c)
	var body struct {
		Asset  string  `json:"asset"`
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Asset == "" || body.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要 asset 和 amount"})
		return
	}
	if err := s.ledger.Deposit(user, body.Asset, body.Amount); err != nil { // [DB:WRITE]
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s.ledger.GetBalances(user))
}

func (s *Server) placeOrder(c *gin.Context) {
	user := userID(c)
	var body struct {
		Side     types.Side `json:"side"`
		Price    float64    `json:"price"`
		Quantity float64    `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 JSON"})
		return
	}
	order, trades, err := s.engine.Place(user, body.Side, body.Price, body.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"order":  order,
		"trades": trades,
	})
}

func (s *Server) openOrders(c *gin.Context) {
	c.JSON(http.StatusOK, s.engine.OpenOrders(userID(c)))
}

func (s *Server) cancelOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效订单 ID"})
		return
	}
	order, err := s.engine.Cancel(userID(c), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

// userID 从请求头取当前用户（演示用）
func userID(c *gin.Context) string {
	u := c.GetHeader("X-User-Id")
	if u == "" {
		u = c.Query("user_id")
	}
	if u == "" {
		u = "alice"
	}
	return u
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-User-Id")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// rateLimitMiddleware [Redis:WRITE] 每 IP 每分钟最多 120 次
func rateLimitMiddleware(rdb *cache.Redis) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()
		ok, err := rdb.Allow(ctx, c.ClientIP(), 120, time.Minute)
		if err == nil && !ok {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁"})
			return
		}
		c.Next()
	}
}
