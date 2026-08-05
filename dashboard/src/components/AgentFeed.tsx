import type { AgentThought, Decision } from "../types";

interface Props {
  thoughts: AgentThought[];
}

function decisionClass(decision: Decision): string {
  if (decision === "BUY") return "decision decision--buy";
  if (decision === "SELL") return "decision decision--sell";
  return "decision decision--hold";
}

function formatOrder(thought: AgentThought): string {
  if (!thought.order) return "—";
  const { accepted, quantity, price, trades } = thought.order;
  return `${accepted ? "OK" : "REJECTED"} ${quantity}@${price} (${trades} fill${trades === 1 ? "" : "s"})`;
}

/** Agent ID | RAG News/Context Query | Structured Decision | Executed Order,
 * newest first, driven by the agent_thoughts Redis channel. */
export function AgentFeed({ thoughts }: Props) {
  return (
    <div className="panel">
      <h2>Agent Thought &amp; Action Feed</h2>
      <table className="feed-table">
        <thead>
          <tr>
            <th>Agent</th>
            <th>Context</th>
            <th>Decision</th>
            <th>Executed Order</th>
          </tr>
        </thead>
        <tbody>
          {thoughts.length === 0 && (
            <tr>
              <td colSpan={4} className="panel__empty">
                Waiting for agent activity…
              </td>
            </tr>
          )}
          {thoughts.map((t, i) => (
            <tr key={`${t.agent_id}-${t.timestamp_unix_ms}-${i}`}>
              <td>{t.agent_id}</td>
              <td className="feed-table__context">{t.context_summary}</td>
              <td>
                <span className={decisionClass(t.decision)}>{t.decision}</span>
                <div className="feed-table__reason">{t.reason}</div>
              </td>
              <td>{formatOrder(t)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
