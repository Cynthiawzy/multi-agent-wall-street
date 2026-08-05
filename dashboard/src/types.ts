export type Side = "BUY" | "SELL";
export type Decision = "BUY" | "SELL" | "HOLD";

export interface Trade {
  symbol: string;
  trade_id: string;
  buy_order_id: string;
  sell_order_id: string;
  price: number;
  quantity: number;
  taker_side: Side;
  timestamp_unix_ms: number;
}

export interface DepthLevel {
  price: number;
  volume: number;
  order_count: number;
}

export interface OrderBookUpdate {
  symbol: string;
  bids: DepthLevel[];
  asks: DepthLevel[];
  total_bid_volume: number;
  total_ask_volume: number;
}

export interface AgentOrder {
  price: number;
  quantity: number;
  accepted: boolean;
  trades: number;
}

export interface AgentThought {
  agent_id: string;
  symbol: string;
  context_summary: string;
  decision: Decision;
  reason: string;
  order: AgentOrder | null;
  timestamp_unix_ms: number;
}

export type ServerEnvelope =
  | { channel: "trades"; payload: Trade }
  | { channel: "orderbook_updates"; payload: OrderBookUpdate }
  | { channel: "agent_thoughts"; payload: AgentThought };
