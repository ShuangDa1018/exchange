package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/demo/dex/internal/auth"
	"github.com/demo/dex/internal/engine"
	"github.com/gorilla/websocket"
)

type Hub struct {
	upgrader websocket.Upgrader
	auth     *auth.Service
	eng      *engine.Engine
	mu       sync.RWMutex
	clients  map[*client]struct{}
}

type client struct {
	conn     *websocket.Conn
	send     chan []byte
	subs     map[string]bool
	userID   string
	lastPong time.Time
}

func New(a *auth.Service, eng *engine.Engine) *Hub {
	h := &Hub{
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		auth:     a,
		eng:      eng,
		clients:  map[*client]struct{}{},
	}
	eng.OnTick(func(symbol string, m *engine.MarketState) {
		h.Broadcast(map[string]any{
			"channel": "ticker", "symbol": symbol,
			"data": map[string]any{
				"price": m.Price.String(), "mark": m.Mark.String(), "index": m.Index.String(),
				"high": m.High.String(), "low": m.Low.String(), "volume": m.Vol.String(),
			},
		})
		h.Broadcast(map[string]any{"channel": "orderbook", "symbol": symbol, "data": m.Book})
		kl := m.Klines["1m"]
		if len(kl) > 0 {
			h.Broadcast(map[string]any{"channel": "kline", "symbol": symbol, "interval": "1m", "data": kl[len(kl)-1]})
		}
	})
	go h.heartbeat()
	return h
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{
		conn: conn, send: make(chan []byte, 64), subs: map[string]bool{}, lastPong: time.Now(),
	}
	if tok := r.URL.Query().Get("token"); tok != "" {
		if uid, err := h.auth.Parse(tok); err == nil {
			c.userID = uid
		}
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	go c.write()
	go h.read(c)
}

func (h *Hub) read(c *client) {
	defer h.drop(c)
	c.conn.SetReadLimit(1 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.lastPong = time.Now()
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg map[string]any
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		op, _ := msg["op"].(string)
		switch op {
		case "ping":
			h.send(c, map[string]any{"op": "pong", "ts": time.Now().UnixMilli()})
		case "subscribe":
			ch, _ := msg["channel"].(string)
			sym, _ := msg["symbol"].(string)
			c.subs[ch+":"+sym] = true
			h.send(c, map[string]any{"op": "subscribed", "channel": ch, "symbol": sym})
		case "unsubscribe":
			ch, _ := msg["channel"].(string)
			sym, _ := msg["symbol"].(string)
			delete(c.subs, ch+":"+sym)
		}
	}
}

func (c *client) write() {
	tk := time.NewTicker(20 * time.Second)
	defer func() {
		tk.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-tk.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte("hb")); err != nil {
				return
			}
		}
	}
}

func (h *Hub) drop(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	c.conn.Close()
}

func (h *Hub) send(c *client, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}

func (h *Hub) Broadcast(v map[string]any) {
	ch, _ := v["channel"].(string)
	sym, _ := v["symbol"].(string)
	key := ch + ":" + sym
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if len(c.subs) == 0 || c.subs[key] || c.subs[ch+":"] {
			select {
			case c.send <- b:
			default:
			}
		}
	}
}

func (h *Hub) heartbeat() {
	tk := time.NewTicker(15 * time.Second)
	defer tk.Stop()
	for range tk.C {
		h.mu.RLock()
		stale := []*client{}
		for c := range h.clients {
			if time.Since(c.lastPong) > 75*time.Second {
				stale = append(stale, c)
			}
		}
		h.mu.RUnlock()
		for _, c := range stale {
			h.drop(c)
		}
	}
}
