import { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { extractErrorMessage } from "../lib/json";

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

const markdownComponents = {
  a: ({ ...props }) => (
    <a
      {...props}
      target="_blank"
      rel="noreferrer"
      className="text-brand-600 underline dark:text-brand-400"
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
      className="rounded bg-slate-100 px-1 py-0.5 text-xs dark:bg-zinc-800"
    />
  ),
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
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Agent
      </h2>
      <div className="space-y-3 rounded-lg border border-slate-200 p-4 dark:border-zinc-800">
        <textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
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

      {error && (
        <div className="rounded-lg border border-red-500 bg-red-50 p-3 text-sm dark:bg-red-950">
          {error}
        </div>
      )}

      {result && (
        <div className="space-y-3 rounded-lg border border-slate-200 p-3 text-sm dark:border-zinc-800">
          <div>
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={markdownComponents}
            >
              {result.answer}
            </ReactMarkdown>
          </div>
          {identity && !chain && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
              <span className="rounded bg-slate-100 px-2 py-1 dark:bg-zinc-800">
                identity: {identity}
              </span>
            </div>
          )}
          {chain && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
              <span className="rounded bg-slate-100 px-2 py-1 dark:bg-zinc-800">
                sub: {chain.sub}
              </span>
              {chain.act && (
                <>
                  <span>→</span>
                  <span className="rounded bg-slate-100 px-2 py-1 dark:bg-zinc-800">
                    act: {chain.act}
                  </span>
                </>
              )}
              {chain.act_act && (
                <>
                  <span>→</span>
                  <span className="rounded bg-slate-100 px-2 py-1 dark:bg-zinc-800">
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
