import { useEffect, useRef } from "react";
import { createChart, type CandlestickData, type IChartApi, type ISeriesApi, type UTCTimestamp } from "lightweight-charts";
import type { Trade } from "../types";

const BUCKET_SECONDS = 5;
const CHART_HEIGHT = 320;

interface Props {
  trades: Trade[];
  symbol: string;
}

function bucketStart(timestampMs: number): UTCTimestamp {
  const bucketMs = BUCKET_SECONDS * 1000;
  return (Math.floor(timestampMs / bucketMs) * BUCKET_SECONDS) as UTCTimestamp;
}

/** Aggregates raw trade prints into fixed-width OHLC candles. The engine
 * emits individual fills, not bars, so this bucketing happens client-side. */
function toCandles(trades: Trade[], symbol: string): CandlestickData[] {
  const bySymbolOldestFirst = trades.filter((t) => t.symbol === symbol).slice().reverse();
  const buckets = new Map<UTCTimestamp, CandlestickData>();

  for (const trade of bySymbolOldestFirst) {
    const time = bucketStart(trade.timestamp_unix_ms);
    const price = trade.price;
    const existing = buckets.get(time);
    if (!existing) {
      buckets.set(time, { time, open: price, high: price, low: price, close: price });
    } else {
      existing.high = Math.max(existing.high, price);
      existing.low = Math.min(existing.low, price);
      existing.close = price;
    }
  }

  return Array.from(buckets.values()).sort((a, b) => (a.time as number) - (b.time as number));
}

export function CandlestickChart({ trades, symbol }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: CHART_HEIGHT,
      layout: { background: { color: "#0b0e11" }, textColor: "#d1d4dc", attributionLogo: false },
      grid: {
        vertLines: { color: "#1b1f27" },
        horzLines: { color: "#1b1f27" },
      },
      timeScale: { timeVisible: true, secondsVisible: true },
    });
    const series = chart.addCandlestickSeries({
      upColor: "#26a69a",
      downColor: "#ef5350",
      borderVisible: false,
      wickUpColor: "#26a69a",
      wickDownColor: "#ef5350",
    });

    chartRef.current = chart;
    seriesRef.current = series;

    const handleResize = () => {
      if (containerRef.current) {
        chart.applyOptions({ width: containerRef.current.clientWidth });
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, []);

  useEffect(() => {
    seriesRef.current?.setData(toCandles(trades, symbol));
  }, [trades, symbol]);

  return (
    <div className="panel">
      <h2>{symbol} — Price</h2>
      <div ref={containerRef} />
    </div>
  );
}
