"""Momentum trading agent: trades on short-term order-book price trend."""

from __future__ import annotations

from collections import deque
from typing import Optional

from agents.base import AgentConfig, OrderIntent, TraderAgent
from agents.grpc_client import MarketClient, MarketDepth


class MomentumAgentConfig(AgentConfig):
    history_window: int = 10
    trend_threshold_ticks: float = 5.0  # minimum price move (ticks) to act on


class MomentumAgent(TraderAgent):
    """Buys when the mid-price is trending up, sells when trending down.

    Trend is the current mid-price compared against the average mid-price
    over the trailing `history_window` ticks. The history buffer lives on
    the agent instance and is only mutated in perceive(), so decide() stays
    a pure function of its context.
    """

    def __init__(self, config: MomentumAgentConfig, client: MarketClient) -> None:
        super().__init__(config, client)
        self.config: MomentumAgentConfig = config
        self._price_history: deque[float] = deque(maxlen=config.history_window)

    async def perceive(self) -> dict:
        depth = await self.client.get_market_depth(self.config.symbol)
        mid_price = self._mid_price(depth)

        history_before = list(self._price_history)
        if mid_price is not None:
            self._price_history.append(mid_price)

        return {"depth": depth, "mid_price": mid_price, "history": history_before}

    async def decide(self, context: dict) -> Optional[OrderIntent]:
        mid_price = context["mid_price"]
        depth: MarketDepth = context["depth"]
        history: list[float] = context["history"]

        if mid_price is None:
            return None  # one side of the book is empty; nothing to trade against
        if len(history) < self.config.history_window:
            return None  # still warming up the trend window

        avg = sum(history) / len(history)
        trend = mid_price - avg

        if trend > self.config.trend_threshold_ticks and depth.asks:
            return OrderIntent(
                side="BUY",
                price=depth.asks[0].price,
                quantity=self.config.order_quantity,
                reason=f"uptrend: mid {mid_price:.1f} vs avg {avg:.1f}",
            )
        if trend < -self.config.trend_threshold_ticks and depth.bids:
            return OrderIntent(
                side="SELL",
                price=depth.bids[0].price,
                quantity=self.config.order_quantity,
                reason=f"downtrend: mid {mid_price:.1f} vs avg {avg:.1f}",
            )
        return None

    @staticmethod
    def _mid_price(depth: MarketDepth) -> Optional[float]:
        if not depth.bids or not depth.asks:
            return None
        return (depth.bids[0].price + depth.asks[0].price) / 2
