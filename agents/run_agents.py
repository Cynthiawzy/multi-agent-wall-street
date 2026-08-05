"""Spawn parallel trading agent loops against the Go MarketService.

Requires a running MarketService gRPC server (see server/grpc_server.go) —
this script only drives the Python agent side.
"""

from __future__ import annotations

import asyncio
import logging
import signal

from redis.asyncio import Redis

from agents.chroma import get_news_collection, seed_sample_headlines
from agents.grpc_client import MarketClient
from agents.momentum_agent import MomentumAgent, MomentumAgentConfig
from agents.news_agent import NewsAgent, NewsAgentConfig

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

GRPC_TARGET = "localhost:50051"
REDIS_URL = "redis://localhost:6379"
TICK_INTERVAL_SECONDS = 2.0

# Anchor prices (integer ticks, e.g. cents) agents fall back to when a book
# has no resting liquidity yet on the side they'd otherwise react to.
REFERENCE_PRICES = {"AAPL": 15000, "TSLA": 25000, "MSFT": 40000}


async def run_agent_loop(agent, stop_event: asyncio.Event) -> None:
    while not stop_event.is_set():
        try:
            await agent.tick()
        except Exception:
            logger.exception("%s: tick failed", agent.config.name)
        await asyncio.sleep(TICK_INTERVAL_SECONDS)


async def main() -> None:
    news_collection = get_news_collection()
    seed_sample_headlines(news_collection)

    redis_client = Redis.from_url(REDIS_URL)
    stop_event = asyncio.Event()

    async with MarketClient(GRPC_TARGET) as client:
        agents = [
            MomentumAgent(
                MomentumAgentConfig(name="momentum-aapl", symbol="AAPL", reference_price=REFERENCE_PRICES["AAPL"]),
                client,
                redis_client,
            ),
            MomentumAgent(
                MomentumAgentConfig(name="momentum-tsla", symbol="TSLA", reference_price=REFERENCE_PRICES["TSLA"]),
                client,
                redis_client,
            ),
            MomentumAgent(
                MomentumAgentConfig(name="momentum-msft", symbol="MSFT", reference_price=REFERENCE_PRICES["MSFT"]),
                client,
                redis_client,
            ),
            NewsAgent(
                NewsAgentConfig(name="news-aapl", symbol="AAPL", reference_price=REFERENCE_PRICES["AAPL"]),
                client,
                news_collection,
                redis_client,
            ),
            NewsAgent(
                NewsAgentConfig(name="news-tsla", symbol="TSLA", reference_price=REFERENCE_PRICES["TSLA"]),
                client,
                news_collection,
                redis_client,
            ),
            NewsAgent(
                NewsAgentConfig(name="news-msft", symbol="MSFT", reference_price=REFERENCE_PRICES["MSFT"]),
                client,
                news_collection,
                redis_client,
            ),
        ]

        loop = asyncio.get_running_loop()
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, stop_event.set)

        logger.info("starting %d agent loops against %s", len(agents), GRPC_TARGET)
        try:
            await asyncio.gather(*(run_agent_loop(a, stop_event) for a in agents))
        finally:
            await redis_client.aclose()


if __name__ == "__main__":
    asyncio.run(main())
