-- ============================================================
-- 002_users：用户账号表（注册 / 登录）
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    user_id       text        PRIMARY KEY,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
