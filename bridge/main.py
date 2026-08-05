"""WebSocket proxy bridge.

Subscribes to the Redis channels the Go engine and Python agents publish to
(trades, orderbook_updates, agent_thoughts) and relays each message, as a
{"channel": ..., "payload": ...} JSON envelope, to every connected browser
WebSocket client.

Run with: uvicorn bridge.main:app --port 8000
"""

from __future__ import annotations

import asyncio
import json
import logging
from contextlib import asynccontextmanager
from typing import AsyncIterator

import redis.asyncio as aioredis
from fastapi import FastAPI, WebSocket, WebSocketDisconnect

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

REDIS_URL = "redis://localhost:6379"
CHANNELS = ("trades", "orderbook_updates", "agent_thoughts")


class ConnectionManager:
    """Tracks connected dashboard clients and fans messages out to all of them."""

    def __init__(self) -> None:
        self._clients: set[WebSocket] = set()

    async def connect(self, ws: WebSocket) -> None:
        await ws.accept()
        self._clients.add(ws)
        logger.info("client connected (%d total)", len(self._clients))

    def disconnect(self, ws: WebSocket) -> None:
        self._clients.discard(ws)
        logger.info("client disconnected (%d total)", len(self._clients))

    async def broadcast(self, message: str) -> None:
        dead: list[WebSocket] = []
        for client in self._clients:
            try:
                await client.send_text(message)
            except Exception:
                dead.append(client)
        for client in dead:
            self.disconnect(client)


manager = ConnectionManager()


async def redis_listener() -> None:
    """Forward every message on CHANNELS to all connected WebSocket clients
    until cancelled. Runs for the lifetime of the FastAPI app."""
    redis_client = aioredis.from_url(REDIS_URL, decode_responses=True)
    pubsub = redis_client.pubsub()
    await pubsub.subscribe(*CHANNELS)
    logger.info("subscribed to redis channels: %s", CHANNELS)

    try:
        async for message in pubsub.listen():
            if message["type"] != "message":
                continue

            channel = message["channel"]
            try:
                payload = json.loads(message["data"])
            except (TypeError, ValueError):
                payload = message["data"]

            envelope = json.dumps({"channel": channel, "payload": payload})
            await manager.broadcast(envelope)
    finally:
        await pubsub.close()
        await redis_client.close()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    listener_task = asyncio.create_task(redis_listener())
    try:
        yield
    finally:
        listener_task.cancel()
        try:
            await listener_task
        except asyncio.CancelledError:
            pass


app = FastAPI(lifespan=lifespan)


@app.websocket("/ws")
async def websocket_endpoint(ws: WebSocket) -> None:
    await manager.connect(ws)
    try:
        while True:
            # Dashboard clients are read-only consumers; this just blocks
            # until the browser closes the connection.
            await ws.receive_text()
    except WebSocketDisconnect:
        manager.disconnect(ws)


@app.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok"}
