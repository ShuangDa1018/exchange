-- [DB:WRITE] 注册用户；冲突则不插入（调用方检查返回行）
INSERT INTO users(user_id, password_hash)
VALUES ($1, $2)
ON CONFLICT (user_id) DO NOTHING
RETURNING user_id;
