-- [DB:WRITE] 写入/更新订单
INSERT INTO orders(id, user_id, symbol, side, price, quantity, filled, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
    filled = EXCLUDED.filled,
    status = EXCLUDED.status;
