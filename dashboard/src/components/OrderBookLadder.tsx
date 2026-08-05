import type { DepthLevel, OrderBookUpdate } from "../types";

interface Props {
  book: OrderBookUpdate | undefined;
  symbol: string;
}

interface LevelWithCumulative extends DepthLevel {
  cumulative: number;
}

function withCumulative(levels: DepthLevel[]): LevelWithCumulative[] {
  let running = 0;
  return levels.map((level) => {
    running += level.volume;
    return { ...level, cumulative: running };
  });
}

function Row({
  level,
  maxCumulative,
  side,
}: {
  level: LevelWithCumulative;
  maxCumulative: number;
  side: "bid" | "ask";
}) {
  const barPct = maxCumulative === 0 ? 0 : (level.cumulative / maxCumulative) * 100;
  return (
    <div className={`ladder-row ladder-row--${side}`}>
      <div className="ladder-row__bar" style={{ width: `${barPct}%` }} />
      <span className="ladder-row__price">{level.price}</span>
      <span className="ladder-row__volume">{level.volume}</span>
      <span className="ladder-row__cumulative">{level.cumulative}</span>
    </div>
  );
}

/** Color-coded depth ladder: bids green (below spread), asks red (above),
 * each row's bar width scaled to its cumulative volume so liquidity
 * concentration is visible at a glance. */
export function OrderBookLadder({ book, symbol }: Props) {
  if (!book) {
    return (
      <div className="panel">
        <h2>{symbol} — Order Book</h2>
        <p className="panel__empty">Waiting for depth updates…</p>
      </div>
    );
  }

  const asks = withCumulative([...book.asks].reverse());
  const bids = withCumulative(book.bids);
  const maxCumulative = Math.max(
    asks[asks.length - 1]?.cumulative ?? 0,
    bids[bids.length - 1]?.cumulative ?? 0,
    1,
  );
  const bestAsk = book.asks[0];
  const bestBid = book.bids[0];

  return (
    <div className="panel">
      <h2>{symbol} — Order Book</h2>
      <div className="ladder">
        <div className="ladder__header">
          <span>Price</span>
          <span>Size</span>
          <span>Total</span>
        </div>
        <div className="ladder__side ladder__side--asks">
          {asks.map((level) => (
            <Row key={`ask-${level.price}`} level={level} maxCumulative={maxCumulative} side="ask" />
          ))}
        </div>
        <div className="ladder__spread">
          {bestAsk && bestBid ? `spread ${bestAsk.price - bestBid.price}` : "—"}
        </div>
        <div className="ladder__side ladder__side--bids">
          {bids.map((level) => (
            <Row key={`bid-${level.price}`} level={level} maxCumulative={maxCumulative} side="bid" />
          ))}
        </div>
      </div>
    </div>
  );
}
