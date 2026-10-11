import { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

type OpsAgentChain = {
  sub?: string;
  act?: string;
  act_act?: string;
};

type OpsAgentResult = {
  answer: string;
  extra?: { chain?: OpsAgentChain } & Record<string, unknown>;
};

const markdownComponents = {
  a: ({ ...props }) => (
    <a
      {...props}
      target="_blank"
      rel="noreferrer"
      className="text-indigo-600 underline dark:text-indigo-400"
    />
  ),
  ul: ({ ...props }) => <ul {...props} className="list-disc space-y-1 pl-5" />,
  ol: ({ ...props }) => (
    <ol {...props} className="list-decimal space-y-1 pl-5" />
  ),
  p: ({ ...props }) => <p {...props} className="[&:not(:first-child)]:mt-2" />,
  code: ({ ...props }) => (
    <code
      {...props}
      className="rounded bg-slate-100 px-1 py-0.5 text-xs dark:bg-slate-800"
    />
  ),
};

export function OpsAgentRunPane({
  identity,
  agent,
  mode,
  defaultPrompt,
}: {
  identity: string;
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
      const res = await fetch("/api/ops-agent/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identity, agent, mode, query }),
      });
      if (!res.ok) {
        setError(await res.text());
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

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Agent
      </h2>
      <div className="space-y-3 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
        <textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          rows={2}
          className="w-full rounded border border-slate-200 bg-transparent p-2 text-sm dark:border-slate-700"
        />
        <button
          onClick={run}
          disabled={running}
          className="rounded bg-slate-900 px-3 py-1 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
        >
          Run agent
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-red-500 bg-red-50 p-3 text-sm dark:bg-red-950">
          {error}
        </div>
      )}

      {result && (
        <div className="space-y-3 rounded-lg border border-slate-200 p-3 text-sm dark:border-slate-800">
          <div>
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={markdownComponents}
            >
              {result.answer}
            </ReactMarkdown>
          </div>
          {chain && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
              <span className="rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                sub: {chain.sub}
              </span>
              {chain.act && (
                <>
                  <span>→</span>
                  <span className="rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                    act: {chain.act}
                  </span>
                </>
              )}
              {chain.act_act && (
                <>
                  <span>→</span>
                  <span className="rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                    act.act: {chain.act_act}
                  </span>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
