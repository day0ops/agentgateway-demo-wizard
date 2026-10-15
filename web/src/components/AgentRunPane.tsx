import { useState } from "react";
import { AnimatePresence, motion } from "framer-motion";

type Hop = { type: string; message: string; detail?: string };

export function AgentRunPane({
  identity,
  model,
}: {
  identity: string;
  model: string;
}) {
  const [prompt, setPrompt] = useState(
    "My private banking advisor mentioned AAPL - what's it trading at right now?",
  );
  const [hops, setHops] = useState<Hop[]>([]);
  const [running, setRunning] = useState(false);

  async function run() {
    setHops([]);
    setRunning(true);

    const res = await fetch("/api/agent/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ identity, prompt, model }),
    });

    if (!res.ok) {
      const message = await res.text();
      setHops((h) => [...h, { type: "error", message }]);
      setRunning(false);
      return;
    }

    if (!res.body) {
      setRunning(false);
      return;
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    try {
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        let sep;
        while ((sep = buffer.indexOf("\n\n")) !== -1) {
          const chunk = buffer.slice(0, sep);
          buffer = buffer.slice(sep + 2);
          const line = chunk.replace(/^data: /, "");
          if (!line) continue;
          setHops((h) => [...h, JSON.parse(line) as Hop]);
        }
      }
    } catch (err) {
      setHops((h) => [
        ...h,
        {
          type: "error",
          message: err instanceof Error ? err.message : String(err),
        },
      ]);
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Agent
      </h2>
      <div className="space-y-3 rounded-lg border border-slate-200 p-4 dark:border-zinc-800">
        <textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          rows={2}
          className="w-full rounded border border-slate-200 bg-transparent p-2 text-sm dark:border-zinc-700"
        />
        <button
          onClick={run}
          disabled={running}
          className="rounded-lg bg-brand-600 hover:bg-brand-700 px-3 py-1 text-sm text-white shadow-sm transition-transform duration-150 ease-out active:scale-[0.97] disabled:opacity-50 disabled:active:scale-100 dark:bg-zinc-100 dark:text-zinc-900"
        >
          Run agent
        </button>
      </div>

      <ol className="space-y-2">
        <AnimatePresence>
          {hops.map((hop, index) => (
            <motion.li
              key={index}
              initial={{ opacity: 0, y: -8 }}
              animate={{ opacity: 1, y: 0 }}
              className={`rounded-lg border p-3 text-sm ${
                hop.type === "error"
                  ? "border-red-500 bg-red-50 dark:bg-red-950"
                  : "border-slate-200 dark:border-zinc-800"
              }`}
            >
              <div className="font-medium">{hop.message}</div>
              {hop.detail && (
                <pre className="mt-1 overflow-x-auto text-xs text-slate-500">
                  {hop.detail}
                </pre>
              )}
            </motion.li>
          ))}
        </AnimatePresence>
      </ol>
    </div>
  );
}
