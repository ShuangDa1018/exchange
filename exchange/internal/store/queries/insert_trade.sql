-- [DB:WRITE] 插入成交
INSERT INTO trades(id, symbol, price, quantity, buy_order_id, sell_order_id, buyer_id, seller_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO NOTHING;
