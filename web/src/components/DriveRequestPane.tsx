import { useState } from "react";
import type { RequestPreset } from "../lib/api";

type RequestState = {
  method: string;
  path: string;
  body: string;
  identity: string;
  headers: Record<string, string>;
  stream: boolean;
};

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
      body: preset.body,
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
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Request &amp; Response
      </h2>

      <div className="space-y-4 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
        <div className="flex flex-wrap gap-2">
          {presets.map((preset) => (
            <button
              key={preset.id}
              onClick={() => loadPreset(preset)}
              className="rounded bg-slate-100 px-3 py-1 text-sm dark:bg-slate-800"
            >
              {preset.title}
            </button>
          ))}
        </div>

        {request && (
          <div className="space-y-2">
            <div className="text-xs text-slate-500">
              {request.method} {request.path} - as {request.identity}
            </div>
            {Object.keys(request.headers).length > 0 && (
              <div className="space-y-0.5 text-xs text-slate-500">
                {Object.entries(request.headers).map(([name, value]) => (
                  <div key={name}>
                    {name}: {value}
                  </div>
                ))}
              </div>
            )}
            <textarea
              value={request.body}
              onChange={(e) => setRequest({ ...request, body: e.target.value })}
              rows={4}
              className="w-full rounded border border-slate-200 bg-transparent p-2 font-mono text-xs dark:border-slate-700"
            />
            <button
              onClick={send}
              disabled={result.status === "sending"}
              className="rounded bg-slate-900 px-3 py-1 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
            >
              Send
            </button>
          </div>
        )}

        {result.status === "error" && (
          <p className="rounded border border-red-500 bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
            {result.message}
          </p>
        )}

        {result.status === "streaming" && (
          <div className="space-y-2 border-t border-slate-200 pt-4 dark:border-slate-800">
            <div className="text-xs text-slate-500">streaming...</div>
            <pre className="overflow-x-auto rounded bg-slate-950 p-3 text-xs text-slate-100">
              {result.body}
            </pre>
          </div>
        )}

        {result.status === "done" && (
          <div className="space-y-2 border-t border-slate-200 pt-4 dark:border-slate-800">
            <div className="text-xs text-slate-500">
              {result.statusCode} · {result.latencyMs}ms
            </div>
            <pre className="overflow-x-auto rounded bg-slate-950 p-3 text-xs text-slate-100">
              {result.body}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}
