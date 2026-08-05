"""News-sentiment trading agent.

Queries the ChromaDB vector store for recent headlines relevant to its
symbol and trades on their average pre-scored sentiment. This agent does
not compute sentiment itself — it aggregates the `sentiment` metadata
field ChromaDB stores alongside each headline (see agents.chroma).
"""

from __future__ import annotations

import asyncio
from typing import Optional

import chromadb
from redis.asyncio import Redis

from agents.base import AgentConfig, OrderIntent, TraderAgent
from agents.grpc_client import MarketClient, MarketDepth


class NewsAgentConfig(AgentConfig):
    buy_sentiment_threshold: float = 0.3
    sell_sentiment_threshold: float = -0.3
    news_lookback: int = 5  # how many top-matching headlines to average


class NewsAgent(TraderAgent):
    """Buys on net-positive recent news sentiment, sells on net-negative."""

    def __init__(
        self,
        config: NewsAgentConfig,
        client: MarketClient,
        news_collection: chromadb.Collection,
        redis_client: Optional[Redis] = None,
    ) -> None:
        super().__init__(config, client, redis_client)
        self.config: NewsAgentConfig = config
        self._news = news_collection

    async def perceive(self) -> dict:
        depth = await self.client.get_market_depth(self.config.symbol)
        query = f"news affecting {self.config.symbol} stock price"
        sentiment = await asyncio.to_thread(self._recent_average_sentiment)

        if sentiment is None:
            summary = f"query='{query}' no headlines on record"
        else:
            summary = f"query='{query}' avg_sentiment={sentiment:.2f}"

        return {"depth": depth, "sentiment": sentiment, "summary": summary}

    async def decide(self, context: dict) -> Optional[OrderIntent]:
        sentiment = context["sentiment"]
        depth: MarketDepth = context["depth"]

        if sentiment is None:
            return None  # no news on record for this symbol

        if sentiment >= self.config.buy_sentiment_threshold:
            price = depth.asks[0].price if depth.asks else self.config.reference_price
            return OrderIntent(
                side="BUY",
                price=price,
                quantity=self.config.order_quantity,
                reason=f"positive sentiment {sentiment:.2f}",
            )
        if sentiment <= self.config.sell_sentiment_threshold:
            price = depth.bids[0].price if depth.bids else self.config.reference_price
            return OrderIntent(
                side="SELL",
                price=price,
                quantity=self.config.order_quantity,
                reason=f"negative sentiment {sentiment:.2f}",
            )
        return None

    def _recent_average_sentiment(self) -> Optional[float]:
        results = self._news.query(
            query_texts=[f"news affecting {self.config.symbol} stock price"],
            where={"symbol": self.config.symbol},
            n_results=self.config.news_lookback,
        )
        metadatas = (results.get("metadatas") or [[]])[0]
        scores: list[float] = []
        for m in metadatas:
            value = m.get("sentiment")
            if isinstance(value, (int, float)):
                scores.append(float(value))
        if not scores:
            return None
        return sum(scores) / len(scores)
