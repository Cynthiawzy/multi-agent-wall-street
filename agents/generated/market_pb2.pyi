from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Side(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SIDE_UNSPECIFIED: _ClassVar[Side]
    BUY: _ClassVar[Side]
    SELL: _ClassVar[Side]

class OrderType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ORDER_TYPE_UNSPECIFIED: _ClassVar[OrderType]
    LIMIT: _ClassVar[OrderType]
    MARKET: _ClassVar[OrderType]
SIDE_UNSPECIFIED: Side
BUY: Side
SELL: Side
ORDER_TYPE_UNSPECIFIED: OrderType
LIMIT: OrderType
MARKET: OrderType

class OrderRequest(_message.Message):
    __slots__ = ("order_id", "symbol", "side", "type", "price", "quantity")
    ORDER_ID_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    SIDE_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PRICE_FIELD_NUMBER: _ClassVar[int]
    QUANTITY_FIELD_NUMBER: _ClassVar[int]
    order_id: str
    symbol: str
    side: Side
    type: OrderType
    price: int
    quantity: int
    def __init__(self, order_id: _Optional[str] = ..., symbol: _Optional[str] = ..., side: _Optional[_Union[Side, str]] = ..., type: _Optional[_Union[OrderType, str]] = ..., price: _Optional[int] = ..., quantity: _Optional[int] = ...) -> None: ...

class CancelRequest(_message.Message):
    __slots__ = ("symbol", "order_id")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    ORDER_ID_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    order_id: str
    def __init__(self, symbol: _Optional[str] = ..., order_id: _Optional[str] = ...) -> None: ...

class Trade(_message.Message):
    __slots__ = ("trade_id", "symbol", "buy_order_id", "sell_order_id", "price", "quantity", "timestamp_unix_ms", "taker_side")
    TRADE_ID_FIELD_NUMBER: _ClassVar[int]
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    BUY_ORDER_ID_FIELD_NUMBER: _ClassVar[int]
    SELL_ORDER_ID_FIELD_NUMBER: _ClassVar[int]
    PRICE_FIELD_NUMBER: _ClassVar[int]
    QUANTITY_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_UNIX_MS_FIELD_NUMBER: _ClassVar[int]
    TAKER_SIDE_FIELD_NUMBER: _ClassVar[int]
    trade_id: str
    symbol: str
    buy_order_id: str
    sell_order_id: str
    price: int
    quantity: int
    timestamp_unix_ms: int
    taker_side: Side
    def __init__(self, trade_id: _Optional[str] = ..., symbol: _Optional[str] = ..., buy_order_id: _Optional[str] = ..., sell_order_id: _Optional[str] = ..., price: _Optional[int] = ..., quantity: _Optional[int] = ..., timestamp_unix_ms: _Optional[int] = ..., taker_side: _Optional[_Union[Side, str]] = ...) -> None: ...

class OrderResponse(_message.Message):
    __slots__ = ("accepted", "error_message", "trades", "remaining_quantity")
    ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    TRADES_FIELD_NUMBER: _ClassVar[int]
    REMAINING_QUANTITY_FIELD_NUMBER: _ClassVar[int]
    accepted: bool
    error_message: str
    trades: _containers.RepeatedCompositeFieldContainer[Trade]
    remaining_quantity: int
    def __init__(self, accepted: _Optional[bool] = ..., error_message: _Optional[str] = ..., trades: _Optional[_Iterable[_Union[Trade, _Mapping]]] = ..., remaining_quantity: _Optional[int] = ...) -> None: ...

class TradeEvent(_message.Message):
    __slots__ = ("trade",)
    TRADE_FIELD_NUMBER: _ClassVar[int]
    trade: Trade
    def __init__(self, trade: _Optional[_Union[Trade, _Mapping]] = ...) -> None: ...

class DepthLevel(_message.Message):
    __slots__ = ("price", "volume", "order_count")
    PRICE_FIELD_NUMBER: _ClassVar[int]
    VOLUME_FIELD_NUMBER: _ClassVar[int]
    ORDER_COUNT_FIELD_NUMBER: _ClassVar[int]
    price: int
    volume: int
    order_count: int
    def __init__(self, price: _Optional[int] = ..., volume: _Optional[int] = ..., order_count: _Optional[int] = ...) -> None: ...

class MarketDepthRequest(_message.Message):
    __slots__ = ("symbol",)
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    def __init__(self, symbol: _Optional[str] = ...) -> None: ...

class MarketDepthResponse(_message.Message):
    __slots__ = ("symbol", "bids", "asks", "total_bid_volume", "total_ask_volume")
    SYMBOL_FIELD_NUMBER: _ClassVar[int]
    BIDS_FIELD_NUMBER: _ClassVar[int]
    ASKS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_BID_VOLUME_FIELD_NUMBER: _ClassVar[int]
    TOTAL_ASK_VOLUME_FIELD_NUMBER: _ClassVar[int]
    symbol: str
    bids: _containers.RepeatedCompositeFieldContainer[DepthLevel]
    asks: _containers.RepeatedCompositeFieldContainer[DepthLevel]
    total_bid_volume: int
    total_ask_volume: int
    def __init__(self, symbol: _Optional[str] = ..., bids: _Optional[_Iterable[_Union[DepthLevel, _Mapping]]] = ..., asks: _Optional[_Iterable[_Union[DepthLevel, _Mapping]]] = ..., total_bid_volume: _Optional[int] = ..., total_ask_volume: _Optional[int] = ...) -> None: ...
