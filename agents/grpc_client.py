"""Async gRPC client wrapping the Go MarketService for Python agents.

Wraps generated protobuf types in plain dataclasses so agent code never
touches market_pb2 messages directly.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import AsyncIterator

import grpc
from google.protobuf import empty_pb2

from agents.generated import market_pb2, market_pb2_grpc

_SIDE_TO_PB = {"BUY": market_pb2.BUY, "SELL": market_pb2.SELL}


@dataclass(frozen=True)
class Trade:
    trade_id: str
    symbol: str
    buy_order_id: str
    sell_order_id: str
    price: int
    quantity: int
    taker_side: str


@dataclass(frozen=True)
class OrderResult:
    accepted: bool
    error_message: str
    trades: list[Trade]
    remaining_quantity: int


@dataclass(frozen=True)
class DepthLevel:
    price: int
    volume: int
    order_count: int


@dataclass(frozen=True)
class MarketDepth:
    symbol: str
    bids: list[DepthLevel]
    asks: list[DepthLevel]
    total_bid_volume: int
    total_ask_volume: int


def _trade_from_pb(t: market_pb2.Trade) -> Trade:
    return Trade(
        trade_id=t.trade_id,
        symbol=t.symbol,
        buy_order_id=t.buy_order_id,
        sell_order_id=t.sell_order_id,
        price=t.price,
        quantity=t.quantity,
        taker_side=market_pb2.Side.Name(t.taker_side),
    )


def _levels_from_pb(levels: list[market_pb2.DepthLevel]) -> list[DepthLevel]:
    return [DepthLevel(price=l.price, volume=l.volume, order_count=l.order_count) for l in levels]


class MarketClient:
    """Async client for the Go MarketService gRPC server.

    Use as an async context manager:

        async with MarketClient("localhost:50051") as client:
            result = await client.place_limit_order("o1", "AAPL", "BUY", 15000, 10)
    """

    def __init__(self, target: str = "localhost:50051") -> None:
        self._target = target
        self._channel: grpc.aio.Channel | None = None
        self._stub: market_pb2_grpc.MarketServiceStub | None = None

    async def __aenter__(self) -> "MarketClient":
        await self.connect()
        return self

    async def __aexit__(self, *exc_info: object) -> None:
        await self.close()

    async def connect(self) -> None:
        self._channel = grpc.aio.insecure_channel(self._target)
        self._stub = market_pb2_grpc.MarketServiceStub(self._channel)

    async def close(self) -> None:
        if self._channel is not None:
            await self._channel.close()
            self._channel = None
            self._stub = None

    def _require_stub(self) -> market_pb2_grpc.MarketServiceStub:
        if self._stub is None:
            raise RuntimeError("MarketClient is not connected; call connect() first")
        return self._stub

    async def place_limit_order(
        self, order_id: str, symbol: str, side: str, price: int, quantity: int
    ) -> OrderResult:
        """Submit a LIMIT order. price/quantity are integer ticks (e.g. cents)."""
        if side not in _SIDE_TO_PB:
            raise ValueError(f"side must be 'BUY' or 'SELL', got {side!r}")

        request = market_pb2.OrderRequest(
            order_id=order_id,
            symbol=symbol,
            side=_SIDE_TO_PB[side],
            type=market_pb2.LIMIT,
            price=price,
            quantity=quantity,
        )
        response = await self._require_stub().SubmitOrder(request)
        return OrderResult(
            accepted=response.accepted,
            error_message=response.error_message,
            trades=[_trade_from_pb(t) for t in response.trades],
            remaining_quantity=response.remaining_quantity,
        )

    async def cancel_order(self, symbol: str, order_id: str) -> OrderResult:
        request = market_pb2.CancelRequest(symbol=symbol, order_id=order_id)
        response = await self._require_stub().CancelOrder(request)
        return OrderResult(
            accepted=response.accepted,
            error_message=response.error_message,
            trades=[],
            remaining_quantity=response.remaining_quantity,
        )

    async def get_market_depth(self, symbol: str) -> MarketDepth:
        request = market_pb2.MarketDepthRequest(symbol=symbol)
        response = await self._require_stub().GetMarketDepth(request)
        return MarketDepth(
            symbol=response.symbol,
            bids=_levels_from_pb(response.bids),
            asks=_levels_from_pb(response.asks),
            total_bid_volume=response.total_bid_volume,
            total_ask_volume=response.total_ask_volume,
        )

    async def stream_trades(self) -> AsyncIterator[Trade]:
        """Yield trades as they occur across all symbols until cancelled."""
        stub = self._require_stub()
        async for event in stub.StreamMarketData(empty_pb2.Empty()):
            yield _trade_from_pb(event.trade)
