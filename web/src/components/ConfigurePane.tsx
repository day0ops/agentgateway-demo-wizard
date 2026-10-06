import { useState } from "react";
import type { Policy } from "../lib/api";

type PolicyState = {
  status: "idle" | "applying" | "applied" | "error";
  message?: string;
  yaml?: string;
  showYaml: boolean;
};

const idleState: PolicyState = { status: "idle", showYaml: false };

export function ConfigurePane({ policies }: { policies: Policy[] }) {
  const [state, setState] = useState<Record<string, PolicyState>>({});

  if (policies.length === 0) return null;

  async function apply(policy: Policy) {
    setState((s) => ({
      ...s,
      [policy.id]: { status: "applying", showYaml: false },
    }));

    const res = await fetch("/api/config/apply", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ policyId: policy.id }),
    });

    if (!res.ok) {
      const message = await res.text();
      setState((s) => ({
        ...s,
        [policy.id]: { status: "error", message, showYaml: false },
      }));
      return;
    }

    if (!res.body) {
      setState((s) => ({
        ...s,
        [policy.id]: {
          status: "error",
          message: "no response body",
          showYaml: false,
        },
      }));
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
          const event = JSON.parse(line) as {
            type: string;
            message: string;
            yaml?: string;
          };

          if (event.type === "error") {
            setState((s) => ({
              ...s,
              [policy.id]: {
                status: "error",
                message: event.message,
                showYaml: false,
              },
            }));
          } else if (event.type === "done") {
            setState((s) => ({
              ...s,
              [policy.id]: {
                status: "applied",
                message: event.message,
                yaml: event.yaml,
                showYaml: true,
              },
            }));
          }
        }
      }
    } catch (err) {
      setState((s) => ({
        ...s,
        [policy.id]: {
          status: "error",
          message: err instanceof Error ? err.message : String(err),
          showYaml: false,
        },
      }));
    }
  }

  async function revert(policy: Policy) {
    const res = await fetch("/api/config/revert", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ policyId: policy.id }),
    });

    if (!res.ok) {
      const { error } = (await res.json()) as { error: string };
      // Keep the prior yaml/applied data around so the Revert button (not Apply)
      // stays visible: the policy is still applied in the cluster, only the
      // revert attempt failed.
      setState((s) => ({
        ...s,
        [policy.id]: {
          ...s[policy.id],
          status: "error",
          message: error,
          showYaml: false,
        },
      }));
      return;
    }

    setState((s) => ({ ...s, [policy.id]: idleState }));
  }

  function toggleYaml(policyId: string) {
    setState((s) => ({
      ...s,
      [policyId]: { ...s[policyId], showYaml: !s[policyId]?.showYaml },
    }));
  }

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Configure
      </h2>
      {policies.map((policy) => {
        const st = state[policy.id] ?? idleState;
        // A failed revert leaves the policy still applied in the cluster, so
        // keep offering Revert (not Apply) as long as applied yaml is present.
        const canRevert =
          st.status === "applied" ||
          (st.status === "error" && Boolean(st.yaml));
        return (
          <div
            key={policy.id}
            className={`rounded-lg border p-4 ${
              st.status === "error"
                ? "border-red-500 bg-red-50 dark:bg-red-950"
                : "border-slate-200 dark:border-slate-800"
            }`}
          >
            <div className="flex items-center justify-between">
              <span className="font-medium">{policy.title}</span>
              <span className="text-xs text-slate-500">
                {st.status === "applied" && "● applied"}
                {st.status === "applying" && "… applying"}
                {st.status === "error" && "● error"}
                {st.status === "idle" && "○ not applied"}
              </span>
            </div>

            {st.status === "error" && (
              <p className="mt-2 text-sm text-red-700 dark:text-red-300">
                {st.message}
              </p>
            )}

            <div className="mt-3 flex gap-2">
              {canRevert ? (
                <button
                  onClick={() => revert(policy)}
                  className="rounded bg-slate-200 px-3 py-1 text-sm dark:bg-slate-800"
                >
                  Revert
                </button>
              ) : (
                <button
                  onClick={() => apply(policy)}
                  disabled={st.status === "applying"}
                  className="rounded bg-slate-900 px-3 py-1 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
                >
                  Apply
                </button>
              )}
              {st.yaml && (
                <button
                  onClick={() => toggleYaml(policy.id)}
                  className="inline-flex items-center gap-1.5 rounded-full border border-indigo-200 bg-indigo-50 px-3 py-1 text-xs font-medium text-indigo-700 transition-colors hover:bg-indigo-100 dark:border-indigo-900 dark:bg-indigo-950 dark:text-indigo-300 dark:hover:bg-indigo-900"
                >
                  <span
                    className={`transition-transform ${st.showYaml ? "rotate-90" : ""}`}
                  >
                    ›
                  </span>
                  {st.showYaml ? "hide YAML" : "view YAML"}
                </button>
              )}
            </div>

            {st.showYaml && st.yaml && (
              <pre className="mt-3 overflow-x-auto rounded bg-slate-950 p-3 text-xs text-slate-100">
                {st.yaml}
              </pre>
            )}
          </div>
        );
      })}
    </div>
  );
}
