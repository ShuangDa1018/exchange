import Decimal from "decimal.js";
import Big from "big.js";

Decimal.set({ precision: 40, rounding: Decimal.ROUND_DOWN });
Big.DP = 12;

export const D = (v: Decimal.Value | undefined | null) => new Decimal(v || 0);

export function fmt(v: Decimal.Value, dp = 4) {
  try {
    return D(v).toFixed(dp);
  } catch {
    return "0";
  }
}

export function fmtUsd(v: Decimal.Value) {
  const n = D(v);
  const dp = n.abs().gte(1000) ? 2 : n.abs().gte(1) ? 4 : 6;
  return n.toFixed(dp);
}

export function notional(price: Decimal.Value, qty: Decimal.Value) {
  return D(price).mul(D(qty));
}

export function marginOf(price: Decimal.Value, qty: Decimal.Value, lev: number) {
  const l = lev > 0 ? lev : 1;
  return notional(price, qty).div(l);
}

export function unrealizedPnl(side: "long" | "short", entry: Decimal.Value, mark: Decimal.Value, qty: Decimal.Value) {
  const q = D(qty);
  return side === "long" ? D(mark).sub(D(entry)).mul(q) : D(entry).sub(D(mark)).mul(q);
}

export function liqPrice(side: "long" | "short", entry: Decimal.Value, margin: Decimal.Value, qty: Decimal.Value, maint = "0.005") {
  const q = D(qty);
  if (q.lte(0)) return D(0);
  const mm = D(entry).mul(D(maint));
  if (side === "long") return D(entry).sub(D(margin).div(q)).add(mm);
  return D(entry).add(D(margin).div(q)).sub(mm);
}

export function roe(pnl: Decimal.Value, margin: Decimal.Value) {
  const m = D(margin);
  if (m.lte(0)) return D(0);
  return D(pnl).div(m).mul(100);
}

export function bigMul(a: string, b: string) {
  return new Big(a || 0).times(b || 0).toString();
}
