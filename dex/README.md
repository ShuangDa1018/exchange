# TF DEX（2026）

混合型交易所 Demo：行情图表、永续合约、资产充提、积分与推荐返佣。前端 React，后端 Go。

## 启动

```bash
# 终端 1
cd dex/backend && go run ./cmd/server

# 终端 2
cd dex/frontend && npm install && npm run dev
```

打开 http://localhost:5173  
演示账号：`demo@tf.dex` / `demo123`  
管理员：`admin@tf.dex` / `admin123`

## 结构

- `backend` Gin REST + WebSocket 撮合/行情/账户
- `frontend` Vite + Chakra UI + React Query + Redux + Decimal.js + TradingView Lightweight Charts datafeed

可选环境变量：

- `VITE_SENTRY_DSN` 开启 Sentry
- `VITE_PRIVY_APP_ID` 接入 Privy（未配则用 EVM/Solana 演示钱包登录）
- `MAINTENANCE=true` 开启维护页
