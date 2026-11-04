import { useState } from "react";
import { Send } from "lucide-react";
import type { RequestPreset } from "../lib/api";
import { prettyPrintJson } from "../lib/json";
import { CodeBlock } from "./CodeBlock";
import { PaneLabel } from "./PaneLabel";
import { btnPrimary, btnSecondary } from "../lib/ui";

type RequestState = {
  method: string;
  path: string;
  body: string;
  identity: string;
  headers: Record<string, string>;
  stream: boolean;
};

const methodBadgeClassName: Record<string, string> = {
  GET: "border border-info-border bg-info-soft text-info",
  POST: "border border-accent bg-accent text-accent-foreground",
};
const defaultMethodBadgeClassName =
  "border border-border bg-secondary text-muted-foreground";

type ResultState =
  | { status: "idle" }
  | { status: "sending" }
  | { status: "streaming"; body: string }
  | { status: "done"; statusCode: number; body: string; latencyMs: number }
  | { status: "error"; message: string };

export function DriveRequestPane({
  presets,
  onResponse,
}: {
  presets: RequestPreset[];
  onResponse?: (body: string) => void;
}) {
  const [request, setRequest] = useState<RequestState | null>(null);
  const [result, setResult] = useState<ResultState>({ status: "idle" });

  if (presets.length === 0) return null;

  function loadPreset(preset: RequestPreset) {
    setRequest({
      method: preset.method,
      path: preset.path,
      body: prettyPrintJson(preset.body),
      identity: preset.identity,
      headers: preset.headers,
      stream: preset.stream,
    });
    setResult({ status: "idle" });
  }

  async function sendStreaming(current: RequestState) {
    try {
      const res = await fetch("/api/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(current),
      });
      if (!res.ok || !res.body) {
        const message = await res.text().catch(() => `HTTP ${res.status}`);
        setResult({ status: "error", message });
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let accumulated = "";
      setResult({ status: "streaming", body: "" });

      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        accumulated += decoder.decode(value, { stream: true });
        setResult({ status: "streaming", body: accumulated });
      }

      setResult({
        status: "done",
        statusCode: res.status,
        body: accumulated,
        latencyMs: 0,
      });
      onResponse?.(accumulated);
    } catch (err) {
      setResult({
        status: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  async function send() {
    if (!request) return;
    setResult({ status: "sending" });

    if (request.stream) {
      await sendStreaming(request);
      return;
    }

    try {
      const res = await fetch("/api/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      });
      if (!res.ok) {
        const body = await res
          .json()
          .catch(() => ({ error: `HTTP ${res.status}` }));
        setResult({
          status: "error",
          message: body.error ?? `HTTP ${res.status}`,
        });
        return;
      }
      const body = await res.json();
      setResult({
        status: "done",
        statusCode: body.statusCode,
        body: body.body,
        latencyMs: body.latencyMs,
      });
      onResponse?.(body.body);
    } catch (err) {
      setResult({
        status: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  return (
    <div className="space-y-4">
      <PaneLabel>Request &amp; response</PaneLabel>

      <div className="space-y-4 rounded-xl border border-border bg-card p-4 shadow-card">
        <div className="flex flex-wrap gap-2">
          {presets.map((preset) => (
            <button
              key={preset.id}
              onClick={() => loadPreset(preset)}
              className={btnSecondary}
            >
              {preset.title}
            </button>
          ))}
        </div>

        {request && (
          <div className="space-y-2">
            <div className="overflow-hidden rounded-lg border border-border">
              <div className="flex flex-wrap items-center gap-2 bg-elevated px-3 py-2">
                <span
                  className={`inline-flex items-center rounded-md px-2 py-0.5 text-xs font-semibold ${
                    methodBadgeClassName[request.method] ??
                    defaultMethodBadgeClassName
                  }`}
                >
                  {request.method}
                </span>
                <span className="font-mono text-xs text-foreground">
                  {request.path}
                </span>
                <span className="text-xs text-subtle">
                  as {request.identity}
                </span>
              </div>
              {Object.keys(request.headers).length > 0 && (
                <div className="border-t border-border px-3 py-2">
                  <div className="text-[10px] font-semibold uppercase tracking-wide text-subtle">
                    Headers
                  </div>
                  <div className="mt-1 divide-y divide-border">
                    {Object.entries(request.headers).map(([name, value]) => (
                      <div
                        key={name}
                        className="flex gap-2 py-1.5 font-mono text-xs"
                      >
                        <span className="text-primary">{name}</span>
                        <span className="text-subtle">:</span>
                        <span className="text-foreground">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              <div className="border-t border-border px-3 py-2">
                <div className="text-[10px] font-semibold uppercase tracking-wide text-subtle">
                  Body
                </div>
                <textarea
                  value={request.body}
                  onChange={(e) =>
                    setRequest({ ...request, body: e.target.value })
                  }
                  rows={Math.min(20, request.body.split("\n").length + 1)}
                  className="mt-1 w-full resize-y rounded-lg border border-input bg-elevated p-3 font-mono text-xs text-foreground"
                  spellCheck={false}
                />
              </div>
            </div>
            <button
              onClick={send}
              disabled={result.status === "sending"}
              className={btnPrimary}
            >
              <Send size={14} />
              Send
            </button>
          </div>
        )}

        {result.status === "error" && (
          <p className="rounded-lg border border-destructive bg-destructive-soft p-3 text-sm text-destructive">
            {result.message}
          </p>
        )}

        {result.status === "streaming" && (
          <div className="space-y-2 border-t border-border pt-4">
            <div className="text-xs text-muted-foreground">streaming...</div>
            <CodeBlock language="json" code={result.body} />
          </div>
        )}

        {result.status === "done" && (
          <div className="space-y-2 border-t border-border pt-4">
            <div className="text-xs tabular-nums text-muted-foreground">
              {result.statusCode} · {result.latencyMs}ms
            </div>
            <CodeBlock language="json" code={prettyPrintJson(result.body)} />
          </div>
        )}
      </div>
    </div>
  );
}
