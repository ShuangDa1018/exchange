package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/demo/dex/internal/auth"
	"github.com/demo/dex/internal/config"
	"github.com/demo/dex/internal/engine"
	"github.com/demo/dex/internal/store"
	"github.com/demo/dex/internal/types"
	"github.com/demo/dex/internal/ws"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	eng   *engine.Engine
	auth  *auth.Service
	hub   *ws.Hub
}

func New(cfg config.Config, st *store.Store, eng *engine.Engine, a *auth.Service, hub *ws.Hub) *gin.Engine {
	s := &Server{cfg: cfg, store: st, eng: eng, auth: a, hub: hub}
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(cfg.AllowOrigins, ","),
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.GET("/api/health", s.health)
	r.GET("/api/status", s.status)
	r.POST("/api/auth/register", s.register)
	r.POST("/api/auth/login", s.login)
	r.POST("/api/auth/wallet", s.wallet)
	r.GET("/api/markets", s.markets)
	r.GET("/api/markets/:symbol/klines", s.klines)
	r.GET("/api/markets/:symbol/orderbook", s.book)
	r.GET("/api/markets/:symbol/trades", s.publicTrades)
	r.GET("/ws", gin.WrapH(hub))

	priv := r.Group("/api")
	priv.Use(a.Middleware())
	{
		priv.GET("/me", s.me)
		priv.GET("/assets", s.assets)
		priv.GET("/ledger", s.ledger)
		priv.POST("/deposit", s.deposit)
		priv.POST("/withdraw", s.withdraw)
		priv.POST("/orders", s.place)
		priv.GET("/orders", s.orders)
		priv.DELETE("/orders/:id", s.cancel)
		priv.GET("/positions", s.positions)
		priv.POST("/positions/:id/close", s.closePos)
		priv.GET("/trades", s.myTrades)
		priv.GET("/points", s.points)
		priv.GET("/referral", s.referral)
		priv.GET("/admin/stats", s.adminStats)
		priv.GET("/admin/users", s.adminUsers)
	}
	return r
}

func (s *Server) health(c *gin.Context) { c.JSON(200, gin.H{"ok": true, "service": "tf-dex"}) }

func (s *Server) status(c *gin.Context) {
	c.JSON(200, gin.H{"maintenance": s.cfg.Maintenance, "version": s.cfg.Version})
}

func (s *Server) register(c *gin.Context) {
	var body struct{ Email, Password, Name, Invite string }
	if err := c.ShouldBindJSON(&body); err != nil || body.Email == "" || len(body.Password) < 6 {
		c.JSON(400, gin.H{"error": "email + password (>=6) required"})
		return
	}
	u, err := s.store.CreateUser(body.Email, body.Password, body.Name, body.Invite)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	tok, _ := s.auth.Issue(u.ID)
	c.JSON(201, gin.H{"token": tok, "user": u})
}

func (s *Server) login(c *gin.Context) {
	var body struct{ Email, Password string }
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid json"})
		return
	}
	u, err := s.store.Login(body.Email, body.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}
	tok, _ := s.auth.Issue(u.ID)
	c.JSON(200, gin.H{"token": tok, "user": u})
}

func (s *Server) wallet(c *gin.Context) {
	var body struct{ Address, Chain, Invite string }
	if err := c.ShouldBindJSON(&body); err != nil || body.Address == "" {
		c.JSON(400, gin.H{"error": "address required"})
		return
	}
	if body.Chain == "" {
		body.Chain = "evm"
	}
	u := s.store.UpsertWallet(body.Address, body.Chain, body.Invite)
	tok, _ := s.auth.Issue(u.ID)
	c.JSON(200, gin.H{"token": tok, "user": u})
}

func (s *Server) me(c *gin.Context) {
	u := s.store.User(auth.UID(c))
	if u == nil {
		c.JSON(401, gin.H{"error": "user missing"})
		return
	}
	c.JSON(200, u)
}

func (s *Server) markets(c *gin.Context) { c.JSON(200, s.eng.Markets()) }

func (s *Server) klines(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "300"))
	c.JSON(200, s.eng.Klines(c.Param("symbol"), c.DefaultQuery("interval", "1m"), limit))
}

func (s *Server) book(c *gin.Context) { c.JSON(200, s.eng.Book(c.Param("symbol"))) }

func (s *Server) publicTrades(c *gin.Context) {
	c.JSON(200, s.store.RecentTrades(c.Param("symbol"), 40))
}

func (s *Server) assets(c *gin.Context) { c.JSON(200, s.store.Balances(auth.UID(c))) }
func (s *Server) ledger(c *gin.Context) { c.JSON(200, s.store.Ledgers(auth.UID(c))) }

func (s *Server) deposit(c *gin.Context) {
	var body struct {
		Asset  string `json:"asset"`
		Amount string `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid json"})
		return
	}
	amt, err := decimal.NewFromString(body.Amount)
	if err != nil || amt.LessThanOrEqual(decimal.Zero) {
		c.JSON(400, gin.H{"error": "invalid amount"})
		return
	}
	if body.Asset == "" {
		body.Asset = "USDT"
	}
	s.store.Credit(auth.UID(c), body.Asset, amt, "deposit", "onchain-sim")
	c.JSON(200, s.store.Balances(auth.UID(c)))
}

func (s *Server) withdraw(c *gin.Context) {
	var body struct {
		Asset, Amount, Address string
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid json"})
		return
	}
	amt, err := decimal.NewFromString(body.Amount)
	if err != nil || amt.LessThanOrEqual(decimal.Zero) {
		c.JSON(400, gin.H{"error": "invalid amount"})
		return
	}
	if err := s.store.Debit(auth.UID(c), body.Asset, amt, "withdraw", body.Address); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, s.store.Balances(auth.UID(c)))
}

func (s *Server) place(c *gin.Context) {
	var body engine.PlaceReq
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid json"})
		return
	}
	if body.Type == "" {
		body.Type = types.OrderMarket
	}
	if body.PosSide == "" {
		if body.Side == types.SideBuy {
			body.PosSide = types.PosLong
		} else {
			body.PosSide = types.PosShort
		}
	}
	o, p, err := s.eng.Place(auth.UID(c), body)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"order": o, "position": p})
}

func (s *Server) orders(c *gin.Context)    { c.JSON(200, s.store.UserOrders(auth.UID(c))) }
func (s *Server) positions(c *gin.Context) { c.JSON(200, s.store.UserPositions(auth.UID(c))) }
func (s *Server) myTrades(c *gin.Context)  { c.JSON(200, s.store.UserTrades(auth.UID(c), 80)) }
func (s *Server) points(c *gin.Context)    { c.JSON(200, s.store.PointsOf(auth.UID(c))) }

func (s *Server) referral(c *gin.Context) {
	u := s.store.User(auth.UID(c))
	c.JSON(200, s.store.ReferralOf(u))
}

func (s *Server) closePos(c *gin.Context) {
	p, err := s.eng.ClosePosition(auth.UID(c), c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, p)
}

func (s *Server) cancel(c *gin.Context) {
	o := s.store.Order(c.Param("id"))
	if o == nil || o.UserID != auth.UID(c) {
		c.JSON(404, gin.H{"error": "order not found"})
		return
	}
	if o.Status == types.StatusFilled {
		c.JSON(400, gin.H{"error": "already filled"})
		return
	}
	o.Status = types.StatusCancelled
	s.store.PutOrder(o)
	c.JSON(200, o)
}

func (s *Server) adminStats(c *gin.Context) {
	u := s.store.User(auth.UID(c))
	if u == nil || !u.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	c.JSON(200, s.store.Stats())
}

func (s *Server) adminUsers(c *gin.Context) {
	u := s.store.User(auth.UID(c))
	if u == nil || !u.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	c.JSON(200, s.store.Users())
}
