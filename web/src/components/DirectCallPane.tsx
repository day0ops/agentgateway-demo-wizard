import { useState } from "react";
import { Send } from "lucide-react";
import type { DirectCallDemo } from "../lib/api";
import { CodeBlock } from "./CodeBlock";

type ResultState =
  | { status: "idle" }
  | { status: "sending" }
  | { status: "done"; statusCode: number; body: string }
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
        statusCode: body.statusCode,
        body: body.body,
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

      {result.status === "done" && (
        <div className="space-y-2">
          <div className="text-xs text-slate-500">{result.statusCode}</div>
          <CodeBlock language="json" code={result.body} />
        </div>
      )}
    </div>
  );
}
