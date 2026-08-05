"""ChromaDB collection setup for news sentiment storage.

There is no news-ingestion pipeline yet (a future phase), so
seed_sample_headlines exists only to give NewsAgent something to read in
the run_agents.py demo.
"""

from __future__ import annotations

import chromadb

COLLECTION_NAME = "news_sentiment"


def get_news_collection(client: chromadb.ClientAPI | None = None) -> chromadb.Collection:
    client = client or chromadb.EphemeralClient()
    return client.get_or_create_collection(COLLECTION_NAME)


def seed_sample_headlines(collection: chromadb.Collection) -> None:
    collection.upsert(
        ids=["seed-1", "seed-2", "seed-3", "seed-4"],
        documents=[
            "AAPL beats quarterly earnings estimates, raises full-year guidance",
            "Analysts upgrade AAPL price target after strong iPhone demand",
            "TSLA recalls vehicles over software defect, shares under pressure",
            "TSLA misses delivery targets amid production slowdown",
        ],
        metadatas=[
            {"symbol": "AAPL", "sentiment": 0.6},
            {"symbol": "AAPL", "sentiment": 0.4},
            {"symbol": "TSLA", "sentiment": -0.5},
            {"symbol": "TSLA", "sentiment": -0.4},
        ],
    )
