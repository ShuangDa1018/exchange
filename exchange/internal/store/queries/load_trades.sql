-- [DB:READ] 全量读成交（启动恢复用）
SELECT id, symbol, price, quantity, buy_order_id, sell_order_id, buyer_id, seller_id, created_at
FROM trades
ORDER BY id ASC;
