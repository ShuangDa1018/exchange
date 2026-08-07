-- [DB:WRITE] 种子演示用户；已存在则跳过
INSERT INTO users(user_id, password_hash)
VALUES ($1, $2)
ON CONFLICT (user_id) DO NOTHING;
