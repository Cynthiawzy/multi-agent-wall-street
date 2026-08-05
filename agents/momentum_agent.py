"""Momentum trading agent: trades on short-term order-book price trend."""

from __future__ import annotations

from collections import deque
from typing import Optional

from redis.asyncio import Redis

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

    def __init__(
        self,
        config: MomentumAgentConfig,
        client: MarketClient,
        redis_client: Optional[Redis] = None,
    ) -> None:
        super().__init__(config, client, redis_client)
        self.config: MomentumAgentConfig = config
        self._price_history: deque[float] = deque(maxlen=config.history_window)

    async def perceive(self) -> dict:
        depth = await self.client.get_market_depth(self.config.symbol)
        mid_price = self._mid_price(depth)

        history_before = list(self._price_history)
        self._price_history.append(mid_price)

        summary = f"mid={mid_price:.1f} history={len(history_before)}/{self.config.history_window}"

        return {
            "depth": depth,
            "mid_price": mid_price,
            "history": history_before,
            "summary": summary,
        }

    async def decide(self, context: dict) -> Optional[OrderIntent]:
        mid_price: float = context["mid_price"]
        depth: MarketDepth = context["depth"]
        history: list[float] = context["history"]

        if len(history) < self.config.history_window:
            return None  # still warming up the trend window

        avg = sum(history) / len(history)
        trend = mid_price - avg

        if trend > self.config.trend_threshold_ticks:
            price = depth.asks[0].price if depth.asks else self.config.reference_price
            return OrderIntent(
                side="BUY",
                price=price,
                quantity=self.config.order_quantity,
                reason=f"uptrend: mid {mid_price:.1f} vs avg {avg:.1f}",
            )
        if trend < -self.config.trend_threshold_ticks:
            price = depth.bids[0].price if depth.bids else self.config.reference_price
            return OrderIntent(
                side="SELL",
                price=price,
                quantity=self.config.order_quantity,
                reason=f"downtrend: mid {mid_price:.1f} vs avg {avg:.1f}",
            )
        return None

    def _mid_price(self, depth: MarketDepth) -> float:
        """True mid when both sides are quoted; otherwise the one quoted
        side, or the agent's reference price if the book is entirely empty.
        Always returns a usable float so trend tracking can start on tick
        one instead of waiting for liquidity nobody will ever provide first.
        """
        if depth.bids and depth.asks:
            return (depth.bids[0].price + depth.asks[0].price) / 2
        if depth.bids:
            return float(depth.bids[0].price)
        if depth.asks:
            return float(depth.asks[0].price)
        return float(self.config.reference_price)
