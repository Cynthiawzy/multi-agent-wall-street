import { useMarketSocket } from "./hooks/useMarketSocket";
import { OrderBookLadder } from "./components/OrderBookLadder";
import { CandlestickChart } from "./components/CandlestickChart";
import { AgentFeed } from "./components/AgentFeed";

// Matches the symbols agents/run_agents.py spawns agents for.
const SYMBOLS = ["AAPL", "TSLA", "MSFT"];

export default function App() {
  const { connected, orderBooks, trades, thoughts } = useMarketSocket();

  return (
    <div className="app">
      <header className="app__header">
        <h1>Simulated Wall Street — Analytics Terminal</h1>
        <span className={`status-dot ${connected ? "status-dot--live" : "status-dot--down"}`}>
          {connected ? "LIVE" : "DISCONNECTED"}
        </span>
      </header>

      <main className="app__grid">
        <section className="app__symbols">
          {SYMBOLS.map((symbol) => (
            <div key={symbol} className="app__symbol-block">
              <CandlestickChart trades={trades} symbol={symbol} />
              <OrderBookLadder book={orderBooks[symbol]} symbol={symbol} />
            </div>
          ))}
        </section>
        <aside className="app__feed">
          <AgentFeed thoughts={thoughts} />
        </aside>
      </main>
    </div>
  );
}
