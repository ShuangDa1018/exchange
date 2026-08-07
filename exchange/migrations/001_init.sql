-- ============================================================
-- 001_init：交易所 Demo 初始表
-- 余额 / 流水 / 订单 / 成交
-- 金额用 numeric，贴近生产（避免 float 精度问题）
-- ============================================================

CREATE TABLE IF NOT EXISTS balances (
    user_id   text           NOT NULL,
    asset     text           NOT NULL,
    available numeric(30, 8) NOT NULL DEFAULT 0,
    locked    numeric(30, 8) NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, asset)
);

CREATE TABLE IF NOT EXISTS balance_ledger (
    id              bigserial      PRIMARY KEY,
    user_id         text           NOT NULL,
    asset           text           NOT NULL,
    delta_available numeric(30, 8) NOT NULL,
    delta_locked    numeric(30, 8) NOT NULL,
    reason          text           NOT NULL,
    ref_id          text           NOT NULL DEFAULT '',
    created_at      timestamptz    NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id         bigint         PRIMARY KEY,
    user_id    text           NOT NULL,
    symbol     text           NOT NULL,
    side       text           NOT NULL,
    price      numeric(30, 8) NOT NULL,
    quantity   numeric(30, 8) NOT NULL,
    filled     numeric(30, 8) NOT NULL,
    status     text           NOT NULL,
    created_at timestamptz    NOT NULL
);

CREATE TABLE IF NOT EXISTS trades (
    id            bigint         PRIMARY KEY,
    symbol        text           NOT NULL,
    price         numeric(30, 8) NOT NULL,
    quantity      numeric(30, 8) NOT NULL,
    buy_order_id  bigint         NOT NULL,
    sell_order_id bigint         NOT NULL,
    buyer_id      text           NOT NULL,
    seller_id     text           NOT NULL,
    created_at    timestamptz    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ledger_user ON balance_ledger (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_user ON orders (user_id, status);
CREATE INDEX IF NOT EXISTS idx_trades_time ON trades (created_at DESC);
