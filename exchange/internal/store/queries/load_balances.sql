-- [DB:READ] 全量读余额（启动恢复用）
SELECT user_id, asset, available, locked
FROM balances;
