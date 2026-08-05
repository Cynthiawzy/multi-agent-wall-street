package server

import (
	"context"
	"encoding/json"
	"log"

	"github.com/cynthia/simulated-wall-street/engine"
)

// Redis pub/sub channel names consumed by the WebSocket bridge (bridge/main.py).
const (
	ChannelTrades           = "trades"
	ChannelOrderBookUpdates = "orderbook_updates"
)

// Publisher publishes a JSON payload to a pub/sub channel. Satisfied by a
// thin adapter over *redis.Client (see cmd/server/main.go) so this package
// never imports the redis client directly, and can run without one (nil
// Publisher is a no-op) for tests or standalone use.
type Publisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}

type tradeEvent struct {
	Symbol          string `json:"symbol"`
	TradeID         string `json:"trade_id"`
	BuyOrderID      string `json:"buy_order_id"`
	SellOrderID     string `json:"sell_order_id"`
	Price           int64  `json:"price"`
	Quantity        int64  `json:"quantity"`
	TakerSide       string `json:"taker_side"`
	TimestampUnixMs int64  `json:"timestamp_unix_ms"`
}

type depthLevelEvent struct {
	Price      int64 `json:"price"`
	Volume     int64 `json:"volume"`
	OrderCount int   `json:"order_count"`
}

type orderBookUpdateEvent struct {
	Symbol         string            `json:"symbol"`
	Bids           []depthLevelEvent `json:"bids"`
	Asks           []depthLevelEvent `json:"asks"`
	TotalBidVolume int64             `json:"total_bid_volume"`
	TotalAskVolume int64             `json:"total_ask_volume"`
}

// publishTrades emits one JSON message per trade to ChannelTrades.
func (s *Server) publishTrades(ctx context.Context, symbol string, trades []*engine.Trade) {
	if s.publisher == nil || len(trades) == 0 {
		return
	}

	for _, t := range trades {
		evt := tradeEvent{
			Symbol:          symbol,
			TradeID:         t.ID,
			BuyOrderID:      t.BuyOrderID,
			SellOrderID:     t.SellOrderID,
			Price:           t.Price,
			Quantity:        t.Quantity,
			TakerSide:       t.TakerSide.String(),
			TimestampUnixMs: t.Timestamp.UnixMilli(),
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			log.Printf("publishTrades: marshal: %v", err)
			continue
		}
		if err := s.publisher.Publish(ctx, ChannelTrades, payload); err != nil {
			log.Printf("publishTrades: publish: %v", err)
		}
	}
}

// publishDepth emits the current book snapshot to ChannelOrderBookUpdates.
// Called after any operation that mutates the book, not just fills, since
// a resting order or a cancel changes depth without producing a trade.
func (s *Server) publishDepth(ctx context.Context, symbol string, depth engine.OrderBookDepth) {
	if s.publisher == nil {
		return
	}

	evt := orderBookUpdateEvent{
		Symbol:         symbol,
		Bids:           toDepthLevelEvents(depth.Bids),
		Asks:           toDepthLevelEvents(depth.Asks),
		TotalBidVolume: depth.TotalBidVolume,
		TotalAskVolume: depth.TotalAskVolume,
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		log.Printf("publishDepth: marshal: %v", err)
		return
	}
	if err := s.publisher.Publish(ctx, ChannelOrderBookUpdates, payload); err != nil {
		log.Printf("publishDepth: publish: %v", err)
	}
}

func toDepthLevelEvents(levels []engine.DepthLevel) []depthLevelEvent {
	out := make([]depthLevelEvent, 0, len(levels))
	for _, l := range levels {
		out = append(out, depthLevelEvent{Price: l.Price, Volume: l.Volume, OrderCount: l.OrderCount})
	}
	return out
}
