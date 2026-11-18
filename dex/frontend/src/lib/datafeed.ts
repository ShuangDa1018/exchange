import type { Candle } from "@/api/client";

export type Bar = { time: number; open: number; high: number; low: number; close: number; volume: number };

type Sub = { symbol: string; interval: string; last: number; cb: (b: Bar) => void };

const cache = new Map<string, Bar[]>();
const subs = new Map<string, Sub>();

export const toBar = (c: Candle): Bar => ({
  time: c.time,
  open: +c.open,
  high: +c.high,
  low: +c.low,
  close: +c.close,
  volume: +c.volume,
});

const key = (s: string, i: string) => `${s}|${i}`;

export const datafeed = {
  cacheKey: key,
  put(symbol: string, interval: string, bars: Bar[]) {
    cache.set(key(symbol, interval), bars);
  },
  get(symbol: string, interval: string) {
    return cache.get(key(symbol, interval)) || [];
  },
  getBars(symbol: string, interval: string) {
    return this.get(symbol, interval);
  },
  subscribeBars(symbol: string, interval: string, cb: (b: Bar) => void) {
    const k = key(symbol, interval);
    const bars = this.get(symbol, interval);
    subs.set(k, { symbol, interval, last: bars.at(-1)?.time || 0, cb });
    return k;
  },
  unsubscribeBars(id: string) {
    subs.delete(id);
  },
  applyKline(symbol: string, interval: string, candle: Candle) {
    const k = key(symbol, interval);
    const bar = toBar(candle);
    const bars = cache.get(k) || [];
    const last = bars[bars.length - 1];
    if (last && last.time === bar.time) bars[bars.length - 1] = bar;
    else bars.push(bar);
    if (bars.length > 1000) bars.splice(0, bars.length - 1000);
    cache.set(k, bars);
    const sub = subs.get(k);
    sub?.cb(bar);
  },
  clearSubs() {
    subs.clear();
  },
};
