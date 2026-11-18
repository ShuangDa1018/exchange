const TOKEN_KEY = "tf_token";

export const getToken = () => localStorage.getItem(TOKEN_KEY) || "";
export const setToken = (t: string) => localStorage.setItem(TOKEN_KEY, t);
export const clearToken = () => localStorage.removeItem(TOKEN_KEY);

async function req<T>(path: string, init: RequestInit = {}, timeout = 12000): Promise<T> {
  const ctrl = new AbortController();
  const id = setTimeout(() => ctrl.abort(), timeout);
  const headers: Record<string, string> = { "Content-Type": "application/json", ...(init.headers as Record<string, string>) };
  const tok = getToken();
  if (tok) headers.Authorization = `Bearer ${tok}`;
  const res = await fetch(path, { ...init, headers, signal: ctrl.signal });
  clearTimeout(id);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data as T;
}

export type User = {
  id: string; email: string; display_name: string; wallet_evm: string; wallet_sol: string;
  referral_code: string; is_admin: boolean;
};
export type Market = {
  symbol: string; base: string; quote: string; price: string; mark_price: string; index_price: string;
  change_24h: string; high_24h: string; low_24h: string; volume_24h: string; max_leverage: number;
  taker_fee: string; funding_rate: string;
};
export type Candle = { time: number; open: string; high: string; low: string; close: string; volume: string };
export type Book = { bids: { price: string; qty: string }[]; asks: { price: string; qty: string }[]; ts: number };
export type Balance = { asset: string; available: string; locked: string };
export type Order = {
  id: string; symbol: string; side: string; pos_side: string; type: string; price: string; qty: string;
  filled_qty: string; leverage: number; status: string; fee: string; created_at: string;
};
export type Position = {
  id: string; symbol: string; side: "long" | "short"; qty: string; entry_price: string; mark_price: string;
  leverage: number; margin: string; unrealized_pnl: string; liq_price: string; roe: string;
};
export type Trade = { id: string; symbol: string; price: string; qty: string; side: string; fee: string; created_at: string };

export const api = {
  status: () => req<{ maintenance: boolean; version: string }>("/api/status"),
  login: (email: string, password: string) => req<{ token: string; user: User }>("/api/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }),
  register: (email: string, password: string, name: string, invite: string) =>
    req<{ token: string; user: User }>("/api/auth/register", { method: "POST", body: JSON.stringify({ email, password, name, invite }) }),
  wallet: (address: string, chain: string, invite: string) =>
    req<{ token: string; user: User }>("/api/auth/wallet", { method: "POST", body: JSON.stringify({ address, chain, invite }) }),
  me: () => req<User>("/api/me"),
  markets: () => req<Market[]>("/api/markets"),
  klines: (symbol: string, interval: string, limit = 300) => req<Candle[]>(`/api/markets/${symbol}/klines?interval=${interval}&limit=${limit}`),
  book: (symbol: string) => req<Book>(`/api/markets/${symbol}/orderbook`),
  publicTrades: (symbol: string) => req<Trade[]>(`/api/markets/${symbol}/trades`),
  assets: () => req<Balance[]>("/api/assets"),
  ledger: () => req<any[]>("/api/ledger"),
  deposit: (asset: string, amount: string) => req("/api/deposit", { method: "POST", body: JSON.stringify({ asset, amount }) }),
  withdraw: (asset: string, amount: string, address: string) => req("/api/withdraw", { method: "POST", body: JSON.stringify({ asset, amount, address }) }),
  place: (body: object) => req("/api/orders", { method: "POST", body: JSON.stringify(body) }),
  orders: () => req<Order[]>("/api/orders"),
  positions: () => req<Position[]>("/api/positions"),
  closePos: (id: string) => req(`/api/positions/${id}/close`, { method: "POST" }),
  trades: () => req<Trade[]>("/api/trades"),
  points: () => req<{ balance: string; volume_usd: string }>("/api/points"),
  referral: () => req<{ code: string; invitees: number; rebate_usdt: string; points: string }>("/api/referral"),
  adminStats: () => req<any>("/api/admin/stats"),
};
