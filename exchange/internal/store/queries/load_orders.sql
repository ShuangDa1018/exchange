-- [DB:READ] 全量读订单（启动恢复用）
SELECT id, user_id, symbol, side, price, quantity, filled, status, created_at
FROM orders
ORDER BY id ASC;
