-- [DB:READ] 按用户查流水
SELECT id, user_id, asset, delta_available, delta_locked, reason, ref_id, created_at
FROM balance_ledger
WHERE user_id = $1
ORDER BY id DESC
LIMIT $2;
