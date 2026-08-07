-- [DB:WRITE] 追加余额流水
INSERT INTO balance_ledger(user_id, asset, delta_available, delta_locked, reason, ref_id)
VALUES ($1, $2, $3, $4, $5, $6);
