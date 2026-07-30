package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func newLimitOrder(id string, side Side, price, qty int64) *Order {
	return &Order{ID: id, Side: side, Type: Limit, Price: price, Quantity: qty}
}

func newMarketOrder(id string, side Side, qty int64) *Order {
	return &Order{ID: id, Side: side, Type: Market, Quantity: qty}
}

func TestProcessOrder_RestsWhenNoCross(t *testing.T) {
	ob := NewOrderBook("TEST")

	trades, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 0 {
		t.Fatalf("expected no trades, got %d", len(trades))
	}

	bestBid, ok := ob.BestBid()
	if !ok || bestBid != 100 {
		t.Fatalf("expected best bid 100, got %d (ok=%v)", bestBid, ok)
	}

	depth := ob.GetDepth()
	if len(depth.Bids) != 1 || depth.Bids[0].Volume != 10 {
		t.Fatalf("unexpected depth: %+v", depth.Bids)
	}
}

func TestProcessOrder_FullFillAtSamePrice(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 100, 10)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trades, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	tr := trades[0]
	if tr.Price != 100 || tr.Quantity != 10 || tr.BuyOrderID != "b1" || tr.SellOrderID != "s1" {
		t.Fatalf("unexpected trade: %+v", tr)
	}

	if _, ok := ob.BestBid(); ok {
		t.Fatalf("expected empty bid side after full fill")
	}
	if _, ok := ob.BestAsk(); ok {
		t.Fatalf("expected empty ask side after full fill")
	}
}

func TestProcessOrder_PartialFillLeavesRemainderOnBook(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 100, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trades, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 12))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 || trades[0].Quantity != 5 {
		t.Fatalf("unexpected trades: %+v", trades)
	}

	bestBid, ok := ob.BestBid()
	if !ok || bestBid != 100 {
		t.Fatalf("expected remainder resting at 100")
	}

	depth := ob.GetDepth()
	if len(depth.Bids) != 1 || depth.Bids[0].Volume != 7 {
		t.Fatalf("expected 7 remaining on bid side, got %+v", depth.Bids)
	}
	if len(depth.Asks) != 0 {
		t.Fatalf("expected ask side empty, got %+v", depth.Asks)
	}
}

func TestProcessOrder_PriceTimePriority(t *testing.T) {
	ob := NewOrderBook("TEST")
	// Two resting sells at the same price; s1 arrived first and must fill
	// completely before s2 gets any of the incoming buy (FIFO).
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 100, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := ob.ProcessOrder(newLimitOrder("s2", Sell, 100, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trades, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 6))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d: %+v", len(trades), trades)
	}
	if trades[0].SellOrderID != "s1" || trades[0].Quantity != 5 {
		t.Fatalf("expected s1 to fill first for its full 5, got %+v", trades[0])
	}
	if trades[1].SellOrderID != "s2" || trades[1].Quantity != 1 {
		t.Fatalf("expected s2 to fill for the remaining 1, got %+v", trades[1])
	}
}

func TestProcessOrder_BestPriceMatchedFirst(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 101, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := ob.ProcessOrder(newLimitOrder("s2", Sell, 100, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trades, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 101, 5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 || trades[0].SellOrderID != "s2" || trades[0].Price != 100 {
		t.Fatalf("expected the cheaper resting ask (100) to be matched first, got %+v", trades)
	}
}

func TestProcessOrder_LimitDoesNotCrossWhenPriceInsufficient(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 105, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trades, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 0 {
		t.Fatalf("expected no trades since bid 100 < ask 105, got %+v", trades)
	}

	bestBid, _ := ob.BestBid()
	if bestBid != 100 {
		t.Fatalf("expected buy order to rest at 100")
	}
}

func TestProcessOrder_MarketOrderConsumesLiquidityAndDropsRemainder(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 100, 5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	trades, err := ob.ProcessOrder(newMarketOrder("b1", Buy, 8))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 || trades[0].Quantity != 5 {
		t.Fatalf("expected market order to consume all 5 resting, got %+v", trades)
	}

	if _, ok := ob.BestAsk(); ok {
		t.Fatalf("expected ask side to be empty after being consumed")
	}
	if _, ok := ob.BestBid(); ok {
		t.Fatalf("market order remainder should not rest on the book")
	}

	depth := ob.GetDepth()
	if len(depth.Bids) != 0 || len(depth.Asks) != 0 {
		t.Fatalf("expected empty book after market sweep, got %+v", depth)
	}
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 10)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ob.CancelOrder("does-not-exist") {
		t.Fatalf("expected cancel of unknown order to return false")
	}

	if !ob.CancelOrder("b1") {
		t.Fatalf("expected cancel of resting order to succeed")
	}
	if ob.CancelOrder("b1") {
		t.Fatalf("expected second cancel of the same order to return false")
	}

	depth := ob.GetDepth()
	if len(depth.Bids) != 0 || depth.TotalBidVolume != 0 {
		t.Fatalf("expected empty bid side after cancel, got %+v", depth)
	}
}

func TestCancelOrder_PartiallyFilledOrderLeavesLevelIntact(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("s1", Sell, 100, 10)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := ob.ProcessOrder(newLimitOrder("s2", Sell, 100, 10)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Partially fill s1, leaving 4 of it resting alongside s2's untouched 10.
	if _, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 6)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ob.CancelOrder("s1") {
		t.Fatalf("expected cancel of partially filled order to succeed")
	}

	depth := ob.GetDepth()
	if len(depth.Asks) != 1 || depth.Asks[0].Volume != 10 {
		t.Fatalf("expected only s2's 10 remaining on the ask side, got %+v", depth.Asks)
	}
}

func TestGetDepth_LimitsToTopTenLevelsButSumsFullVolume(t *testing.T) {
	ob := NewOrderBook("TEST")
	for i := int64(0); i < 15; i++ {
		id := fmt.Sprintf("b%d", i)
		// Distinct price per level; higher i means a higher (better) bid.
		if _, err := ob.ProcessOrder(newLimitOrder(id, Buy, 100+i, 1)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	depth := ob.GetDepth()
	if len(depth.Bids) != 10 {
		t.Fatalf("expected depth capped at 10 levels, got %d", len(depth.Bids))
	}
	if depth.TotalBidVolume != 15 {
		t.Fatalf("expected total volume across all 15 levels, got %d", depth.TotalBidVolume)
	}
	if depth.Bids[0].Price != 114 {
		t.Fatalf("expected best bid (114) first, got %d", depth.Bids[0].Price)
	}
}

func TestProcessOrder_Validation(t *testing.T) {
	ob := NewOrderBook("TEST")

	cases := []struct {
		name  string
		order *Order
		want  error
	}{
		{"nil order", nil, ErrNilOrder},
		{"empty id", &Order{ID: "", Side: Buy, Type: Limit, Price: 100, Quantity: 1}, ErrEmptyOrderID},
		{"zero quantity", &Order{ID: "x1", Side: Buy, Type: Limit, Price: 100, Quantity: 0}, ErrInvalidQuantity},
		{"negative quantity", &Order{ID: "x2", Side: Buy, Type: Limit, Price: 100, Quantity: -1}, ErrInvalidQuantity},
		{"zero price for limit", &Order{ID: "x3", Side: Buy, Type: Limit, Price: 0, Quantity: 1}, ErrInvalidPrice},
		{"invalid side", &Order{ID: "x4", Side: Side(99), Type: Limit, Price: 100, Quantity: 1}, ErrInvalidSide},
		{"invalid type", &Order{ID: "x5", Side: Buy, Type: OrderType(99), Price: 100, Quantity: 1}, ErrInvalidType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ob.ProcessOrder(tc.order)
			if err == nil || !errors.Is(err, tc.want) {
				t.Fatalf("expected error wrapping %v, got %v", tc.want, err)
			}
		})
	}
}

func TestProcessOrder_DuplicateOrderID(t *testing.T) {
	ob := NewOrderBook("TEST")
	if _, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := ob.ProcessOrder(newLimitOrder("b1", Buy, 100, 1))
	if err == nil || !errors.Is(err, ErrDuplicateOrderID) {
		t.Fatalf("expected ErrDuplicateOrderID, got %v", err)
	}
}

// TestOrderBook_ConcurrentAccess exercises the RWMutex under concurrent
// ProcessOrder calls from many goroutines. Run with `go test -race` to
// actually catch data races; this also sanity-checks the book stays
// internally consistent (no negative volumes) under contention.
func TestOrderBook_ConcurrentAccess(t *testing.T) {
	ob := NewOrderBook("TEST")
	const n = 200

	var wg sync.WaitGroup
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, _ = ob.ProcessOrder(newLimitOrder(fmt.Sprintf("buy-%d", i), Buy, 100+int64(i%5), 1))
		}(i)
		go func(i int) {
			defer wg.Done()
			_, _ = ob.ProcessOrder(newLimitOrder(fmt.Sprintf("sell-%d", i), Sell, 100+int64(i%5), 1))
		}(i)
	}
	wg.Wait()

	depth := ob.GetDepth()
	for _, lvl := range depth.Bids {
		if lvl.Volume < 0 {
			t.Fatalf("negative bid volume: %+v", lvl)
		}
	}
	for _, lvl := range depth.Asks {
		if lvl.Volume < 0 {
			t.Fatalf("negative ask volume: %+v", lvl)
		}
	}
}
