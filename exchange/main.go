// 入口文件：组装各模块并启动 HTTP 服务
// 运行：cd exchange && go run .
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	// 本项目内部包（go.mod 里的 module 名 + 路径）
	"github.com/demo/exchange/internal/account"
	"github.com/demo/exchange/internal/api"
	"github.com/demo/exchange/internal/engine"
)

// //go:embed 是编译器指令（不是普通注释）
// 作用：把 static/ 打进二进制，部署不用再单独拷前端文件
// ≈ webpack/vite 把静态资源打包进产物
//go:embed static/*
var staticFS embed.FS

func main() {
	// 1) 账本：管余额
	ledger := account.NewLedger()
	// 预置两个用户，方便页面上切换 alice/bob 对敲
	// map[string]float64 ≈ Record<string, number>
	ledger.EnsureUser("alice", map[string]float64{"USDT": 10000, "BTC": 1})
	ledger.EnsureUser("bob", map[string]float64{"USDT": 10000, "BTC": 1})

	// 2) 撮合引擎：管下单/成交
	// BTC-USDT：base=BTC（买卖标的），quote=USDT（计价货币）
	eng := engine.New("BTC-USDT", "BTC", "USDT", ledger)

	// 3) API 层：把上面两层暴露成 HTTP
	apiServer := api.New(ledger, eng)

	// 4) 静态前端页面（index.html）
	web, err := fs.Sub(staticFS, "static") // 取出 static 子目录当网站根
	if err != nil {
		log.Fatal(err) // 打印并退出进程
	}
	fileServer := http.FileServer(http.FS(web)) // ≈ express.static('static')

	// 手动分流：/api/* → 后端；其它 → 前端静态文件
	// （Go 1.22+ 的 ServeMux 里 "/api/" 和 "GET /" 会冲突，所以不用自动挂载）
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path ≈ location.pathname
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	addr := ":8080" // 省略 host = 监听所有网卡的 8080
	log.Printf("交易所 Demo 已启动: http://localhost%s", addr)
	log.Printf("交易对: BTC-USDT | 用户: alice / bob（请求头 X-User-Id）")
	log.Printf("  GET  /api/book")
	log.Printf("  GET  /api/trades")
	log.Printf("  GET  /api/balances")
	log.Printf("  POST /api/orders")
	log.Printf("  DELETE /api/orders/{id}")

	// ListenAndServe 会一直阻塞，像 app.listen()
	if err := http.ListenAndServe(addr, withCORS(root)); err != nil {
		log.Fatal(err)
	}
}

// withCORS 中间件：给所有响应加跨域头
// ≈ express 的 cors()；写法是“包一层 next”
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
