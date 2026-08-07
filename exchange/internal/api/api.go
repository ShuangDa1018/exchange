// Package api：HTTP 接口层（Gin）
// 只负责：解析请求 → 调 engine/account → 返回 JSON
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/auth"
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
	auth   *auth.Service
	router *gin.Engine
}

func New(ledger *account.Ledger, eng *engine.Engine, rdb *cache.Redis, authSvc *auth.Service) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(rateLimitMiddleware(rdb)) // [Redis:WRITE] 限流
	r.Use(authSvc.Middleware())     // JWT 或 X-User-Id

	s := &Server{ledger: ledger, engine: eng, rdb: rdb, auth: authSvc, router: r}
	s.routes()
	return s
}

func (s *Server) Engine() *gin.Engine { return s.router }

func (s *Server) routes() {
	api := s.router.Group("/api")
	{
		api.GET("/health", s.health)
		api.POST("/login", s.login)       // 学习：换 JWT
		api.POST("/register", s.register) // 学习：注册并换 JWT

		api.GET("/book", s.book)                 // [MEM]
		api.GET("/trades", s.trades)             // [MEM]
		api.GET("/balances", s.balances)         // [Redis:READ]
		api.GET("/ledger", s.ledgerEntries)      // [DB:READ]
		api.POST("/deposit", s.deposit)          // [DB:WRITE]
		api.POST("/orders", s.placeOrder)        // [MEM]+[DB:WRITE]+[Kafka]
		api.GET("/orders", s.openOrders)         // [MEM]
		api.DELETE("/orders/:id", s.cancelOrder) // [MEM]+[DB:WRITE]
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "symbol": s.engine.Symbol()})
}

// POST /api/login {"user_id":"alice","password":"alice"}
func (s *Server) login(c *gin.Context) {
	var body struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要 user_id 和 password"})
		return
	}
	token, err := s.auth.Login(body.UserID, body.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": body.UserID})
}

// POST /api/register {"user_id":"carol","password":"carol"}
func (s *Server) register(c *gin.Context) {
	var body struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要 user_id 和 password"})
		return
	}
	token, err := s.auth.Register(body.UserID, body.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 演示：新用户给一点种子资金，方便立刻下单
	s.ledger.EnsureUser(body.UserID, map[string]float64{"USDT": 10000, "BTC": 1})
	c.JSON(http.StatusCreated, gin.H{"token": token, "user_id": body.UserID})
}

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
	c.JSON(http.StatusOK, s.ledger.GetBalances(auth.UserID(c)))
}

func (s *Server) ledgerEntries(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	entries, err := s.ledger.ListLedger(auth.UserID(c), limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (s *Server) deposit(c *gin.Context) {
	user := auth.UserID(c)
	var body struct {
		Asset  string  `json:"asset"`
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Asset == "" || body.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要 asset 和 amount"})
		return
	}
	if err := s.ledger.Deposit(user, body.Asset, body.Amount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s.ledger.GetBalances(user))
}

func (s *Server) placeOrder(c *gin.Context) {
	user := auth.UserID(c)
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
	c.JSON(http.StatusCreated, gin.H{"order": order, "trades": trades})
}

func (s *Server) openOrders(c *gin.Context) {
	c.JSON(http.StatusOK, s.engine.OpenOrders(auth.UserID(c)))
}

func (s *Server) cancelOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效订单 ID"})
		return
	}
	order, err := s.engine.Cancel(auth.UserID(c), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-User-Id, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func rateLimitMiddleware(rdb *cache.Redis) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if rdb == nil || path == "/api/login" || path == "/api/register" {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()
		// 前端约 4 路轮询；演示环境给宽一点，避免正常刷新就 429
		ok, err := rdb.Allow(ctx, c.ClientIP(), 600, time.Minute)
		if err == nil && !ok {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁"})
			return
		}
		c.Next()
	}
}
