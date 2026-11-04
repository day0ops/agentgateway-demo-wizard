import { useState } from "react";
import { Send } from "lucide-react";
import type { DirectCallDemo } from "../lib/api";
import { interpretToolCall, type DirectCallOutcome } from "../lib/mcp";
import { btnSecondary } from "../lib/ui";

type ResultState =
  | { status: "idle" }
  | { status: "sending" }
  | { status: "done"; outcome: DirectCallOutcome }
  | { status: "error"; message: string };

export function DirectCallPane({ config }: { config: DirectCallDemo }) {
  const [result, setResult] = useState<ResultState>({ status: "idle" });

  async function send() {
    setResult({ status: "sending" });
    try {
      const res = await fetch("/secure/direct-call", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          path: config.path,
          tool: config.tool,
          arguments: config.arguments,
        }),
      });
      if (!res.ok) {
        setResult({ status: "error", message: await res.text() });
        return;
      }
      const body = await res.json();
      setResult({
        status: "done",
        outcome: interpretToolCall(body.statusCode, body.body),
      });
    } catch (err) {
      setResult({
        status: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <div className="space-y-3 rounded-xl border border-border bg-card p-4 shadow-card">
      <button
        onClick={send}
        disabled={result.status === "sending"}
        className={btnSecondary}
      >
        <Send size={14} />
        {config.title}
      </button>

      {result.status === "error" && (
        <div className="rounded-lg border border-destructive bg-destructive-soft p-3 text-sm text-destructive">
          {result.message}
        </div>
      )}

      {result.status === "done" && result.outcome.kind === "success" && (
        <div className="space-y-1 rounded-lg border border-success-border bg-success-soft p-3 text-sm">
          <div className="font-medium text-success">
            Allowed - the transfer went through
          </div>
          <pre className="whitespace-pre-wrap font-mono text-xs text-foreground">
            {result.outcome.text}
          </pre>
        </div>
      )}

      {result.status === "done" && result.outcome.kind === "denied" && (
        <div className="space-y-1 rounded-lg border border-warning-border bg-warning-soft p-3 text-sm">
          <div className="font-medium text-warning">Denied by policy</div>
          <div className="text-foreground">
            The{" "}
            <code className="rounded bg-warning/15 px-1 py-0.5 font-mono text-xs text-warning">
              {config.tool}
            </code>{" "}
            tool isn't available to this direct, non-delegated call - the
            gateway hides it unless the request arrives through the proper
            delegation chain.
          </div>
          <div className="font-mono text-xs text-warning">
            {result.outcome.detail}
          </div>
        </div>
      )}

      {result.status === "done" && result.outcome.kind === "error" && (
        <div className="rounded-lg border border-destructive bg-destructive-soft p-3 text-sm text-destructive">
          {result.outcome.detail}
        </div>
      )}
    </div>
  );
}
