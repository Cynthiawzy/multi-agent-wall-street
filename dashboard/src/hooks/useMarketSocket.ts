import { useEffect, useRef, useState } from "react";
import type { AgentThought, OrderBookUpdate, ServerEnvelope, Trade } from "../types";

const WS_URL = "ws://localhost:8000/ws";
const MAX_TRADES = 300;
const MAX_THOUGHTS = 100;
const RECONNECT_DELAY_MS = 2000;

export interface MarketSocketState {
  connected: boolean;
  orderBooks: Record<string, OrderBookUpdate>;
  trades: Trade[];
  thoughts: AgentThought[];
}

/** Connects to the FastAPI bridge, reconnecting on drop, and accumulates the
 * three event streams (order book snapshots, trade prints, agent thoughts)
 * into state components can render directly. */
export function useMarketSocket(): MarketSocketState {
  const [connected, setConnected] = useState(false);
  const [orderBooks, setOrderBooks] = useState<Record<string, OrderBookUpdate>>({});
  const [trades, setTrades] = useState<Trade[]>([]);
  const [thoughts, setThoughts] = useState<AgentThought[]>([]);
  const socketRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    let cancelled = false;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;

    const connect = () => {
      const ws = new WebSocket(WS_URL);
      socketRef.current = ws;

      ws.onopen = () => setConnected(true);

      ws.onclose = () => {
        setConnected(false);
        if (!cancelled) {
          reconnectTimer = setTimeout(connect, RECONNECT_DELAY_MS);
        }
      };

      ws.onerror = () => ws.close();

      ws.onmessage = (event: MessageEvent<string>) => {
        let envelope: ServerEnvelope;
        try {
          envelope = JSON.parse(event.data) as ServerEnvelope;
        } catch {
          return;
        }

        switch (envelope.channel) {
          case "orderbook_updates":
            setOrderBooks((prev) => ({ ...prev, [envelope.payload.symbol]: envelope.payload }));
            break;
          case "trades":
            setTrades((prev) => [envelope.payload, ...prev].slice(0, MAX_TRADES));
            break;
          case "agent_thoughts":
            setThoughts((prev) => [envelope.payload, ...prev].slice(0, MAX_THOUGHTS));
            break;
        }
      };
    };

    connect();

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimer);
      socketRef.current?.close();
    };
  }, []);

  return { connected, orderBooks, trades, thoughts };
}
