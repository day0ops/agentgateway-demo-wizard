import { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { extractErrorMessage } from "../lib/json";
import { markdownComponents } from "../lib/markdown";
import { PaneLabel } from "./PaneLabel";
import { btnPrimary, inputBase } from "../lib/ui";

const chainChip =
  "rounded-md border border-border bg-secondary px-2 py-1 font-mono text-secondary-foreground";

type OpsAgentChain = {
  sub?: string;
  act?: string;
  act_act?: string;
};

type OpsAgentResult = {
  answer: string;
  extra?: { chain?: OpsAgentChain; identity?: string } & Record<
    string,
    unknown
  >;
};

export function OpsAgentRunPane({
  agent,
  mode,
  defaultPrompt,
}: {
  agent: string;
  mode: string;
  defaultPrompt: string;
}) {
  const [query, setQuery] = useState(defaultPrompt);
  const [result, setResult] = useState<OpsAgentResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  async function run() {
    setRunning(true);
    setError(null);
    setResult(null);

    try {
      const res = await fetch("/secure/ops-agent/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ agent, mode, query }),
      });
      if (!res.ok) {
        setError(await extractErrorMessage(res));
        return;
      }
      setResult((await res.json()) as OpsAgentResult);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setRunning(false);
    }
  }

  const chain = result?.extra?.chain;
  const identity = result?.extra?.identity;

  return (
    <div className="space-y-4">
      <PaneLabel>Agent</PaneLabel>
      <div className="space-y-3 rounded-xl border border-border bg-card p-4 shadow-card">
        <textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          rows={2}
          className={`${inputBase} w-full resize-y`}
        />
        <button onClick={run} disabled={running} className={btnPrimary}>
          Run agent
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-destructive bg-destructive-soft p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {result && (
        <div className="space-y-3 rounded-xl border border-border bg-card p-4 text-sm shadow-card">
          <div className="leading-relaxed text-foreground">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={markdownComponents}
            >
              {result.answer}
            </ReactMarkdown>
          </div>
          {identity && !chain && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className={chainChip}>identity: {identity}</span>
            </div>
          )}
          {chain && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className={chainChip}>sub: {chain.sub}</span>
              {chain.act && (
                <>
                  <span className="text-subtle">→</span>
                  <span className={chainChip}>act: {chain.act}</span>
                </>
              )}
              {chain.act_act && (
                <>
                  <span className="text-subtle">→</span>
                  <span className={chainChip}>act.act: {chain.act_act}</span>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
