// Package server implements the MarketService gRPC contract defined in
// proto/market.proto, wrapping one engine.OrderBook per symbol.
package server

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/cynthia/simulated-wall-street/engine"
	pb "github.com/cynthia/simulated-wall-street/proto/marketpb"
)

// subscriberBuffer bounds how many undelivered trade events a single
// streaming client can lag behind by before events start being dropped for
// it. Buffered + non-blocking send keeps a slow dashboard from ever stalling
// the matching engine.
const subscriberBuffer = 64

// Server implements pb.MarketServiceServer. engine.OrderBook is already
// internally thread-safe (see engine/orderbook.go), so Server does not wrap
// individual OrderBook calls in a lock. What it does need to protect is its
// own state: the registry mapping symbol -> OrderBook (lazily created, and
// reachable from many concurrent RPC goroutines) and the set of currently
// connected StreamMarketData subscribers.
type Server struct {
	pb.UnimplementedMarketServiceServer

	booksMu sync.RWMutex
	books   map[string]*engine.OrderBook

	subMu       sync.Mutex
	subscribers map[chan *pb.TradeEvent]struct{}

	// publisher fans trades/depth out to Redis for the WebSocket bridge
	// (bridge/main.py). Nil is a valid no-op publisher.
	publisher Publisher
}

func NewServer(publisher Publisher) *Server {
	return &Server{
		books:       make(map[string]*engine.OrderBook),
		subscribers: make(map[chan *pb.TradeEvent]struct{}),
		publisher:   publisher,
	}
}

// bookFor returns the OrderBook for symbol, creating it on first use.
// Double-checked locking: the common case (book already exists) only takes
// a read lock, so concurrent requests for already-known symbols never
// contend with each other. The write lock is only taken the first time a
// given symbol is seen.
func (s *Server) bookFor(symbol string) *engine.OrderBook {
	s.booksMu.RLock()
	book, ok := s.books[symbol]
	s.booksMu.RUnlock()
	if ok {
		return book
	}

	s.booksMu.Lock()
	defer s.booksMu.Unlock()
	if book, ok = s.books[symbol]; ok {
		return book // another goroutine created it while we waited for the write lock
	}
	book = engine.NewOrderBook(symbol)
	s.books[symbol] = book
	return book
}

func (s *Server) SubmitOrder(ctx context.Context, req *pb.OrderRequest) (*pb.OrderResponse, error) {
	order, err := toEngineOrder(req)
	if err != nil {
		return &pb.OrderResponse{Accepted: false, ErrorMessage: err.Error()}, nil
	}

	book := s.bookFor(req.GetSymbol())
	trades, err := book.ProcessOrder(order)
	if err != nil {
		return &pb.OrderResponse{Accepted: false, ErrorMessage: err.Error()}, nil
	}

	s.broadcastTrades(req.GetSymbol(), trades)
	s.publishTrades(ctx, req.GetSymbol(), trades)
	s.publishDepth(ctx, req.GetSymbol(), book.GetDepth())

	return &pb.OrderResponse{
		Accepted:          true,
		Trades:            toPBTrades(req.GetSymbol(), trades),
		RemainingQuantity: order.Quantity,
	}, nil
}

func (s *Server) CancelOrder(ctx context.Context, req *pb.CancelRequest) (*pb.OrderResponse, error) {
	book := s.bookFor(req.GetSymbol())
	if !book.CancelOrder(req.GetOrderId()) {
		return &pb.OrderResponse{Accepted: false, ErrorMessage: "order not found"}, nil
	}
	s.publishDepth(ctx, req.GetSymbol(), book.GetDepth())
	return &pb.OrderResponse{Accepted: true}, nil
}

func (s *Server) GetMarketDepth(_ context.Context, req *pb.MarketDepthRequest) (*pb.MarketDepthResponse, error) {
	book := s.bookFor(req.GetSymbol())
	return toPBDepth(req.GetSymbol(), book.GetDepth()), nil
}

// StreamMarketData registers a subscriber channel and forwards trade events
// to it until the client disconnects or the stream's context is canceled.
func (s *Server) StreamMarketData(_ *emptypb.Empty, stream pb.MarketService_StreamMarketDataServer) error {
	ch := make(chan *pb.TradeEvent, subscriberBuffer)

	s.subMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subMu.Unlock()

	defer func() {
		s.subMu.Lock()
		delete(s.subscribers, ch)
		s.subMu.Unlock()
	}()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case evt := <-ch:
			if err := stream.Send(evt); err != nil {
				return err
			}
		}
	}
}

// broadcastTrades fans a batch of fills out to every connected streaming
// subscriber. Sends are non-blocking: a subscriber that isn't draining its
// channel fast enough has events dropped rather than stalling matching for
// everyone else.
func (s *Server) broadcastTrades(symbol string, trades []*engine.Trade) {
	if len(trades) == 0 {
		return
	}

	s.subMu.Lock()
	defer s.subMu.Unlock()

	for _, tr := range trades {
		evt := toPBTradeEvent(symbol, tr)
		for ch := range s.subscribers {
			select {
			case ch <- evt:
			default:
				// Slow consumer; drop this event for it rather than block.
			}
		}
	}
}

func toEngineSide(s pb.Side) (engine.Side, error) {
	switch s {
	case pb.Side_BUY:
		return engine.Buy, nil
	case pb.Side_SELL:
		return engine.Sell, nil
	default:
		return 0, fmt.Errorf("side must be BUY or SELL, got %s", s)
	}
}

func toEngineType(t pb.OrderType) (engine.OrderType, error) {
	switch t {
	case pb.OrderType_LIMIT:
		return engine.Limit, nil
	case pb.OrderType_MARKET:
		return engine.Market, nil
	default:
		return 0, fmt.Errorf("order type must be LIMIT or MARKET, got %s", t)
	}
}

func toPBSide(s engine.Side) pb.Side {
	if s == engine.Sell {
		return pb.Side_SELL
	}
	return pb.Side_BUY
}

func toEngineOrder(req *pb.OrderRequest) (*engine.Order, error) {
	side, err := toEngineSide(req.GetSide())
	if err != nil {
		return nil, err
	}
	orderType, err := toEngineType(req.GetType())
	if err != nil {
		return nil, err
	}
	return &engine.Order{
		ID:       req.GetOrderId(),
		Side:     side,
		Type:     orderType,
		Price:    req.GetPrice(),
		Quantity: req.GetQuantity(),
	}, nil
}

func toPBTrade(symbol string, t *engine.Trade) *pb.Trade {
	return &pb.Trade{
		TradeId:         t.ID,
		Symbol:          symbol,
		BuyOrderId:      t.BuyOrderID,
		SellOrderId:     t.SellOrderID,
		Price:           t.Price,
		Quantity:        t.Quantity,
		TimestampUnixMs: t.Timestamp.UnixMilli(),
		TakerSide:       toPBSide(t.TakerSide),
	}
}

func toPBTrades(symbol string, trades []*engine.Trade) []*pb.Trade {
	out := make([]*pb.Trade, 0, len(trades))
	for _, t := range trades {
		out = append(out, toPBTrade(symbol, t))
	}
	return out
}

func toPBTradeEvent(symbol string, t *engine.Trade) *pb.TradeEvent {
	return &pb.TradeEvent{Trade: toPBTrade(symbol, t)}
}

func toPBLevels(levels []engine.DepthLevel) []*pb.DepthLevel {
	out := make([]*pb.DepthLevel, 0, len(levels))
	for _, l := range levels {
		out = append(out, &pb.DepthLevel{Price: l.Price, Volume: l.Volume, OrderCount: int32(l.OrderCount)})
	}
	return out
}

func toPBDepth(symbol string, d engine.OrderBookDepth) *pb.MarketDepthResponse {
	return &pb.MarketDepthResponse{
		Symbol:         symbol,
		Bids:           toPBLevels(d.Bids),
		Asks:           toPBLevels(d.Asks),
		TotalBidVolume: d.TotalBidVolume,
		TotalAskVolume: d.TotalAskVolume,
	}
}
