-- [DB:READ] 登录查密码哈希
SELECT password_hash
FROM users
WHERE user_id = $1;
