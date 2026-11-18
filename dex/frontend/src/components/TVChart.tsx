import { useEffect, useRef } from "react";
import { Box } from "@chakra-ui/react";
import { createChart, ColorType, type IChartApi, type ISeriesApi, type CandlestickData, type UTCTimestamp } from "lightweight-charts";
import { datafeed, toBar, type Bar } from "@/lib/datafeed";
import { api } from "@/api/client";

export function TVChart({ symbol }: { symbol: string }) {
  const wrap = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const subRef = useRef<string>("");

  useEffect(() => {
    if (!wrap.current) return;
    const chart = createChart(wrap.current, {
      layout: { background: { type: ColorType.Solid, color: "#0d1218" }, textColor: "#9aa4af" },
      grid: { vertLines: { color: "#1b242e" }, horzLines: { color: "#1b242e" } },
      rightPriceScale: { borderColor: "#1b242e" },
      timeScale: { borderColor: "#1b242e", timeVisible: true, secondsVisible: false },
      crosshair: { mode: 0 },
      height: wrap.current.clientHeight || 420,
      autoSize: true,
    });
    const series = chart.addCandlestickSeries({
      upColor: "#17c784", downColor: "#f6465d", borderVisible: false, wickUpColor: "#17c784", wickDownColor: "#f6465d",
    });
    chartRef.current = chart;
    seriesRef.current = series;
    const ro = new ResizeObserver(() => chart.applyOptions({ width: wrap.current?.clientWidth }));
    ro.observe(wrap.current);
    return () => {
      ro.disconnect();
      datafeed.unsubscribeBars(subRef.current);
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, []);

  useEffect(() => {
    const series = seriesRef.current;
    if (!series) return;
    datafeed.unsubscribeBars(subRef.current);
    const apply = (bars: Bar[]) => {
      series.setData(bars.map((b) => ({
        time: b.time as UTCTimestamp, open: b.open, high: b.high, low: b.low, close: b.close,
      })));
    };
    const cached = datafeed.getBars(symbol, "1m");
    if (cached.length) apply(cached);
    const load = async () => {
      const raw = await api.klines(symbol, "1m", 400);
      const bars = raw.map(toBar);
      datafeed.put(symbol, "1m", bars);
      apply(bars);
    };
    if (!cached.length) void load();
    else void load();
    subRef.current = datafeed.subscribeBars(symbol, "1m", (bar) => {
      const d: CandlestickData = { time: bar.time as UTCTimestamp, open: bar.open, high: bar.high, low: bar.low, close: bar.close };
      series.update(d);
    });
    return () => datafeed.unsubscribeBars(subRef.current);
  }, [symbol]);

  return <Box ref={wrap} h={{ base: "280px", md: "460px" }} w="100%" borderRadius="md" overflow="hidden" />;
}
