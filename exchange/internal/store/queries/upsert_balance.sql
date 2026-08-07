-- [DB:WRITE] 写入/更新余额快照
INSERT INTO balances(user_id, asset, available, locked)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, asset) DO UPDATE SET
    available = EXCLUDED.available,
    locked = EXCLUDED.locked;
