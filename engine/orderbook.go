package engine

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const maxDepthLevels = 10

var (
	ErrNilOrder         = errors.New("order is nil")
	ErrEmptyOrderID     = errors.New("order id must not be empty")
	ErrDuplicateOrderID = errors.New("duplicate order id")
	ErrInvalidSide      = errors.New("invalid order side")
	ErrInvalidType      = errors.New("invalid order type")
	ErrInvalidQuantity  = errors.New("order quantity must be positive")
	ErrInvalidPrice     = errors.New("limit order price must be positive")
)

type Side int

const (
	Buy Side = iota
	Sell
)

func (s Side) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

type OrderType int

const (
	Limit OrderType = iota
	Market
)

func (t OrderType) String() string {
	switch t {
	case Limit:
		return "LIMIT"
	case Market:
		return "MARKET"
	default:
		return "UNKNOWN"
	}
}

// Order is a resting or incoming instruction. Price/Quantity are expressed
// in integer ticks (e.g. cents, shares) to avoid float rounding in matching.
type Order struct {
	ID               string
	Side             Side
	Type             OrderType
	Price            int64 // ignored for Market orders
	Quantity         int64 // remaining, mutated in place as fills occur
	OriginalQuantity int64
	Timestamp        time.Time

	// Intrusive doubly-linked list pointers within the owning PriceLevel.
	prev, next *Order
	level      *PriceLevel
}

// PriceLevel is a FIFO queue of resting orders at a single price.
type PriceLevel struct {
	Price  int64
	Volume int64 // sum of remaining Quantity across all orders at this level
	count  int
	head   *Order
	tail   *Order
}

func newPriceLevel(price int64) *PriceLevel {
	return &PriceLevel{Price: price}
}

func (pl *PriceLevel) pushBack(o *Order) {
	o.level = pl
	o.prev = pl.tail
	o.next = nil
	if pl.tail != nil {
		pl.tail.next = o
	} else {
		pl.head = o
	}
	pl.tail = o
	pl.Volume += o.Quantity
	pl.count++
}

// unlink removes o from the list and decrements count only. Callers are
// responsible for adjusting Volume themselves, since the correct delta
// depends on context (a fill already decremented it; a cancel has not).
func (pl *PriceLevel) unlink(o *Order) {
	if o.prev != nil {
		o.prev.next = o.next
	} else {
		pl.head = o.next
	}
	if o.next != nil {
		o.next.prev = o.prev
	} else {
		pl.tail = o.prev
	}
	o.prev, o.next, o.level = nil, nil, nil
	pl.count--
}

type Trade struct {
	ID          string
	BuyOrderID  string
	SellOrderID string
	Price       int64
	Quantity    int64
	Timestamp   time.Time
	TakerSide   Side
}

type DepthLevel struct {
	Price      int64
	Volume     int64
	OrderCount int
}

type OrderBookDepth struct {
	Bids           []DepthLevel // best-first, at most maxDepthLevels
	Asks           []DepthLevel // best-first, at most maxDepthLevels
	TotalBidVolume int64        // across the entire book, not just the levels returned
	TotalAskVolume int64
}

// OrderBook is a single-symbol price-time priority limit order book.
// All exported methods are safe for concurrent use.
type OrderBook struct {
	mu     sync.RWMutex
	Symbol string

	bids      map[int64]*PriceLevel
	asks      map[int64]*PriceLevel
	bidPrices []int64 // sorted descending (best bid first)
	askPrices []int64 // sorted ascending (best ask first)

	orders   map[string]*Order // orderID -> order, for O(1) cancel lookup
	tradeSeq uint64
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		bids:   make(map[int64]*PriceLevel),
		asks:   make(map[int64]*PriceLevel),
		orders: make(map[string]*Order),
	}
}

func validateOrder(order *Order) error {
	if order == nil {
		return ErrNilOrder
	}
	if order.ID == "" {
		return ErrEmptyOrderID
	}
	if order.Side != Buy && order.Side != Sell {
		return fmt.Errorf("order %s: %w", order.ID, ErrInvalidSide)
	}
	if order.Type != Limit && order.Type != Market {
		return fmt.Errorf("order %s: %w", order.ID, ErrInvalidType)
	}
	if order.Quantity <= 0 {
		return fmt.Errorf("order %s: %w", order.ID, ErrInvalidQuantity)
	}
	if order.Type == Limit && order.Price <= 0 {
		return fmt.Errorf("order %s: %w", order.ID, ErrInvalidPrice)
	}
	return nil
}

// ProcessOrder matches the incoming order against the opposite side of the
// book and, for Limit orders with an unfilled remainder, rests it on the
// book. Market orders that cannot be fully filled have their remainder
// dropped, since they carry no price to rest at.
func (ob *OrderBook) ProcessOrder(order *Order) ([]*Trade, error) {
	if err := validateOrder(order); err != nil {
		return nil, err
	}

	ob.mu.Lock()
	defer ob.mu.Unlock()

	if _, exists := ob.orders[order.ID]; exists {
		return nil, fmt.Errorf("order %s: %w", order.ID, ErrDuplicateOrderID)
	}
	if order.Timestamp.IsZero() {
		order.Timestamp = time.Now().UTC()
	}
	order.OriginalQuantity = order.Quantity

	trades := ob.match(order)

	if order.Quantity > 0 && order.Type == Limit {
		ob.addToBook(order)
	}

	return trades, nil
}

func (ob *OrderBook) match(taker *Order) []*Trade {
	var trades []*Trade

	if taker.Side == Buy {
		for taker.Quantity > 0 && len(ob.askPrices) > 0 {
			bestAsk := ob.askPrices[0]
			if taker.Type == Limit && bestAsk > taker.Price {
				break
			}
			level := ob.asks[bestAsk]
			trades = append(trades, ob.matchLevel(taker, level)...)
			if level.head == nil {
				delete(ob.asks, bestAsk)
				ob.askPrices = removeAt(ob.askPrices, 0)
			}
		}
	} else {
		for taker.Quantity > 0 && len(ob.bidPrices) > 0 {
			bestBid := ob.bidPrices[0]
			if taker.Type == Limit && bestBid < taker.Price {
				break
			}
			level := ob.bids[bestBid]
			trades = append(trades, ob.matchLevel(taker, level)...)
			if level.head == nil {
				delete(ob.bids, bestBid)
				ob.bidPrices = removeAt(ob.bidPrices, 0)
			}
		}
	}

	return trades
}

// matchLevel fills taker against resting orders at level in FIFO order.
func (ob *OrderBook) matchLevel(taker *Order, level *PriceLevel) []*Trade {
	var trades []*Trade

	for level.head != nil && taker.Quantity > 0 {
		maker := level.head
		qty := min64(taker.Quantity, maker.Quantity)

		taker.Quantity -= qty
		maker.Quantity -= qty
		level.Volume -= qty

		ob.tradeSeq++
		trade := &Trade{
			ID:        fmt.Sprintf("T-%d", ob.tradeSeq),
			Price:     level.Price,
			Quantity:  qty,
			Timestamp: time.Now().UTC(),
			TakerSide: taker.Side,
		}
		if taker.Side == Buy {
			trade.BuyOrderID, trade.SellOrderID = taker.ID, maker.ID
		} else {
			trade.BuyOrderID, trade.SellOrderID = maker.ID, taker.ID
		}
		trades = append(trades, trade)

		if maker.Quantity == 0 {
			level.unlink(maker)
			delete(ob.orders, maker.ID)
		}
	}

	return trades
}

func (ob *OrderBook) addToBook(order *Order) {
	var level *PriceLevel
	var ok bool

	if order.Side == Buy {
		level, ok = ob.bids[order.Price]
		if !ok {
			level = newPriceLevel(order.Price)
			ob.bids[order.Price] = level
			ob.bidPrices = insertSorted(ob.bidPrices, order.Price, descending)
		}
	} else {
		level, ok = ob.asks[order.Price]
		if !ok {
			level = newPriceLevel(order.Price)
			ob.asks[order.Price] = level
			ob.askPrices = insertSorted(ob.askPrices, order.Price, ascending)
		}
	}

	level.pushBack(order)
	ob.orders[order.ID] = order
}

// CancelOrder removes a resting order from the book. Returns false if no
// order with that ID is currently resting (already filled or unknown).
func (ob *OrderBook) CancelOrder(orderID string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	order, ok := ob.orders[orderID]
	if !ok {
		return false
	}

	level := order.level
	level.Volume -= order.Quantity
	level.unlink(order)
	delete(ob.orders, orderID)

	if level.head == nil {
		if order.Side == Buy {
			delete(ob.bids, level.Price)
			ob.bidPrices = removeSorted(ob.bidPrices, level.Price, descending)
		} else {
			delete(ob.asks, level.Price)
			ob.askPrices = removeSorted(ob.askPrices, level.Price, ascending)
		}
	}

	return true
}

// GetDepth returns up to the top 10 price levels per side (best first) plus
// total resting volume across the whole book on each side.
func (ob *OrderBook) GetDepth() OrderBookDepth {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	depth := OrderBookDepth{
		Bids: make([]DepthLevel, 0, maxDepthLevels),
		Asks: make([]DepthLevel, 0, maxDepthLevels),
	}

	for i, price := range ob.bidPrices {
		level := ob.bids[price]
		depth.TotalBidVolume += level.Volume
		if i < maxDepthLevels {
			depth.Bids = append(depth.Bids, DepthLevel{Price: price, Volume: level.Volume, OrderCount: level.count})
		}
	}
	for i, price := range ob.askPrices {
		level := ob.asks[price]
		depth.TotalAskVolume += level.Volume
		if i < maxDepthLevels {
			depth.Asks = append(depth.Asks, DepthLevel{Price: price, Volume: level.Volume, OrderCount: level.count})
		}
	}

	return depth
}

// BestBid returns the highest resting bid price, if any.
func (ob *OrderBook) BestBid() (int64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.bidPrices) == 0 {
		return 0, false
	}
	return ob.bidPrices[0], true
}

// BestAsk returns the lowest resting ask price, if any.
func (ob *OrderBook) BestAsk() (int64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.askPrices) == 0 {
		return 0, false
	}
	return ob.askPrices[0], true
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

type sortOrder bool

const (
	ascending  sortOrder = true
	descending sortOrder = false
)

func insertSorted(prices []int64, price int64, order sortOrder) []int64 {
	idx := searchInsertIdx(prices, price, order)
	prices = append(prices, 0)
	copy(prices[idx+1:], prices[idx:])
	prices[idx] = price
	return prices
}

func removeSorted(prices []int64, price int64, order sortOrder) []int64 {
	idx := searchInsertIdx(prices, price, order)
	if idx >= len(prices) || prices[idx] != price {
		return prices // not found; no-op
	}
	return append(prices[:idx], prices[idx+1:]...)
}

func searchInsertIdx(prices []int64, price int64, order sortOrder) int {
	if order == ascending {
		return sort.Search(len(prices), func(i int) bool { return prices[i] >= price })
	}
	return sort.Search(len(prices), func(i int) bool { return prices[i] <= price })
}

func removeAt(prices []int64, idx int) []int64 {
	return append(prices[:idx], prices[idx+1:]...)
}
