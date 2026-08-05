"""Spawn parallel trading agent loops against the Go MarketService.

Requires a running MarketService gRPC server (see server/grpc_server.go) —
this script only drives the Python agent side.
"""

from __future__ import annotations

import asyncio
import logging
import signal

from agents.chroma import get_news_collection, seed_sample_headlines
from agents.grpc_client import MarketClient
from agents.momentum_agent import MomentumAgent, MomentumAgentConfig
from agents.news_agent import NewsAgent, NewsAgentConfig

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

GRPC_TARGET = "localhost:50051"
TICK_INTERVAL_SECONDS = 2.0


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

    stop_event = asyncio.Event()

    async with MarketClient(GRPC_TARGET) as client:
        agents = [
            MomentumAgent(MomentumAgentConfig(name="momentum-aapl", symbol="AAPL"), client),
            MomentumAgent(MomentumAgentConfig(name="momentum-tsla", symbol="TSLA"), client),
            MomentumAgent(MomentumAgentConfig(name="momentum-msft", symbol="MSFT"), client),
            NewsAgent(NewsAgentConfig(name="news-aapl", symbol="AAPL"), client, news_collection),
            NewsAgent(NewsAgentConfig(name="news-tsla", symbol="TSLA"), client, news_collection),
        ]

        loop = asyncio.get_running_loop()
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, stop_event.set)

        logger.info("starting %d agent loops against %s", len(agents), GRPC_TARGET)
        await asyncio.gather(*(run_agent_loop(a, stop_event) for a in agents))


if __name__ == "__main__":
    asyncio.run(main())
