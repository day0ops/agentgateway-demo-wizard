import { useState } from "react";
import { Send } from "lucide-react";
import type { DirectCallDemo } from "../lib/api";
import { interpretToolCall, type DirectCallOutcome } from "../lib/mcp";

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
    <div className="space-y-3 rounded-lg border border-slate-200 p-4 dark:border-zinc-800">
      <button
        onClick={send}
        disabled={result.status === "sending"}
        className="inline-flex items-center gap-1.5 rounded-lg bg-slate-200 px-3 py-1 text-sm shadow-sm transition-transform duration-150 ease-out active:scale-[0.97] disabled:opacity-50 disabled:active:scale-100 dark:bg-zinc-800"
      >
        <Send size={14} />
        {config.title}
      </button>

      {result.status === "error" && (
        <div className="rounded-lg border border-red-500 bg-red-50 p-3 text-sm dark:bg-red-950">
          {result.message}
        </div>
      )}

      {result.status === "done" && result.outcome.kind === "success" && (
        <div className="space-y-1 rounded-lg border border-green-500 bg-green-50 p-3 text-sm dark:bg-green-950">
          <div className="font-medium text-green-800 dark:text-green-300">
            Allowed - the transfer went through
          </div>
          <pre className="whitespace-pre-wrap font-mono text-xs text-green-900 dark:text-green-200">
            {result.outcome.text}
          </pre>
        </div>
      )}

      {result.status === "done" && result.outcome.kind === "denied" && (
        <div className="space-y-1 rounded-lg border border-amber-500 bg-amber-50 p-3 text-sm dark:bg-amber-950">
          <div className="font-medium text-amber-800 dark:text-amber-300">
            Denied by policy
          </div>
          <div className="text-amber-900 dark:text-amber-200">
            The{" "}
            <code className="rounded bg-amber-100 px-1 py-0.5 text-xs dark:bg-amber-900">
              {config.tool}
            </code>{" "}
            tool isn't available to this direct, non-delegated call - the
            gateway hides it unless the request arrives through the proper
            delegation chain.
          </div>
          <div className="font-mono text-xs text-amber-700 dark:text-amber-400">
            {result.outcome.detail}
          </div>
        </div>
      )}

      {result.status === "done" && result.outcome.kind === "error" && (
        <div className="rounded-lg border border-red-500 bg-red-50 p-3 text-sm dark:bg-red-950">
          {result.outcome.detail}
        </div>
      )}
    </div>
  );
}
