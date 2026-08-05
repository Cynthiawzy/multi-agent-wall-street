"""Base LangGraph-driven trading agent.

Each tick runs a three-node graph: perceive -> decide -> act. Subclasses
implement perceive() (gather whatever market/domain context the strategy
needs) and decide() (turn that context into a structured OrderIntent, or
None to hold). act() and the graph wiring are shared by every agent.
"""

from __future__ import annotations

import logging
import uuid
from abc import ABC, abstractmethod
from typing import Optional, TypedDict

from langgraph.graph import END, StateGraph
from pydantic import BaseModel, Field, field_validator

from agents.grpc_client import MarketClient, OrderResult

logger = logging.getLogger(__name__)


class AgentConfig(BaseModel):
    """Static configuration shared by every trading agent."""

    name: str
    symbol: str
    order_quantity: int = Field(gt=0, default=10)


class OrderIntent(BaseModel):
    """Structured tool input: the trade a strategy has decided to place."""

    side: str
    price: int = Field(gt=0)
    quantity: int = Field(gt=0)
    reason: str

    @field_validator("side")
    @classmethod
    def side_must_be_valid(cls, v: str) -> str:
        if v not in ("BUY", "SELL"):
            raise ValueError("side must be 'BUY' or 'SELL'")
        return v


class AgentState(TypedDict, total=False):
    context: dict
    intent: Optional[OrderIntent]
    result: Optional[OrderResult]


class TraderAgent(ABC):
    """Base class for a single-symbol trading agent."""

    def __init__(self, config: AgentConfig, client: MarketClient) -> None:
        self.config = config
        self.client = client
        self._graph = self._build_graph()

    def _build_graph(self):
        graph = StateGraph(AgentState)
        graph.add_node("perceive", self._perceive_node)
        graph.add_node("decide", self._decide_node)
        graph.add_node("act", self._act_node)
        graph.set_entry_point("perceive")
        graph.add_edge("perceive", "decide")
        graph.add_edge("decide", "act")
        graph.add_edge("act", END)
        return graph.compile()

    async def _perceive_node(self, state: AgentState) -> AgentState:
        return {"context": await self.perceive()}

    async def _decide_node(self, state: AgentState) -> AgentState:
        return {"intent": await self.decide(state["context"])}

    async def _act_node(self, state: AgentState) -> AgentState:
        intent = state.get("intent")
        if intent is None:
            return {"result": None}

        order_id = f"{self.config.name}-{uuid.uuid4().hex[:8]}"
        result = await self.client.place_limit_order(
            order_id=order_id,
            symbol=self.config.symbol,
            side=intent.side,
            price=intent.price,
            quantity=intent.quantity,
        )
        logger.info(
            "%s: %s %d@%d (%s) -> accepted=%s trades=%d",
            self.config.name,
            intent.side,
            intent.quantity,
            intent.price,
            intent.reason,
            result.accepted,
            len(result.trades),
        )
        return {"result": result}

    @abstractmethod
    async def perceive(self) -> dict:
        """Gather whatever context this agent's decision needs."""

    @abstractmethod
    async def decide(self, context: dict) -> Optional[OrderIntent]:
        """Turn context into an order, or None to hold this tick."""

    async def tick(self) -> AgentState:
        """Run one perceive -> decide -> act cycle."""
        return await self._graph.ainvoke({})
