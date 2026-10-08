import { useState } from "react";
import { Play, Undo2, Eye, EyeOff } from "lucide-react";
import type { Policy } from "../lib/api";
import { StatusBadge } from "./StatusBadge";
import { CodeBlock } from "./CodeBlock";

type PolicyState = {
  status: "idle" | "applying" | "applied" | "error";
  message?: string;
  yaml?: string;
  showYaml: boolean;
};

const idleState: PolicyState = { status: "idle", showYaml: false };

type ViewState = {
  status: "idle" | "loading" | "shown" | "not-provisioned" | "error";
  refs?: { name: string; found: boolean; yaml?: string }[];
  message?: string;
};

const idleViewState: ViewState = { status: "idle" };

export function ConfigurePane({ policies }: { policies: Policy[] }) {
  const [state, setState] = useState<Record<string, PolicyState>>({});
  const [viewState, setViewState] = useState<Record<string, ViewState>>({});

  if (policies.length === 0) return null;

  async function viewConfig(policy: Policy) {
    setViewState((s) => ({ ...s, [policy.id]: { status: "loading" } }));

    const res = await fetch(`/api/config/view?policyId=${policy.id}`);
    if (!res.ok) {
      const message = await res.text();
      setViewState((s) => ({
        ...s,
        [policy.id]: { status: "error", message },
      }));
      return;
    }

    const { refs } = (await res.json()) as {
      refs: { name: string; found: boolean; yaml?: string }[];
    };
    const anyFound = refs.some((r) => r.found);
    setViewState((s) => ({
      ...s,
      [policy.id]: { status: anyFound ? "shown" : "not-provisioned", refs },
    }));
  }

  function hideConfig(policyId: string) {
    setViewState((s) => ({ ...s, [policyId]: idleViewState }));
  }

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
        if (policy.readOnly) {
          const vst = viewState[policy.id] ?? idleViewState;
          return (
            <div
              key={policy.id}
              className="rounded-xl border border-slate-200 p-4 shadow-sm dark:border-slate-800"
            >
              <div className="flex items-center justify-between">
                <span className="font-medium">{policy.title}</span>
                <span className="text-xs text-slate-500">pre-provisioned</span>
              </div>

              {vst.status === "error" && (
                <p className="mt-2 text-sm text-red-700 dark:text-red-300">
                  {vst.message}
                </p>
              )}
              {vst.status === "not-provisioned" && (
                <p className="mt-2 text-sm text-amber-700 dark:text-amber-300">
                  Not yet provisioned - deploy the field-kit usecase for this
                  demo first.
                </p>
              )}

              <div className="mt-3 flex gap-2">
                {vst.status === "shown" ? (
                  <button
                    onClick={() => hideConfig(policy.id)}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-slate-200 px-3 py-1 text-sm shadow-sm dark:bg-slate-800"
                  >
                    <EyeOff size={14} />
                    Hide config
                  </button>
                ) : (
                  <button
                    onClick={() => viewConfig(policy)}
                    disabled={vst.status === "loading"}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
                  >
                    <Eye size={14} />
                    View config
                  </button>
                )}
              </div>

              {vst.status === "shown" &&
                vst.refs
                  ?.filter((r) => r.found)
                  .map((r) => (
                    <div key={r.name} className="mt-3">
                      <CodeBlock code={r.yaml ?? ""} />
                    </div>
                  ))}
            </div>
          );
        }

        const st = state[policy.id] ?? idleState;
        // A failed revert leaves the policy still applied in the cluster, so
        // keep offering Revert (not Apply) as long as applied yaml is present.
        const canRevert =
          st.status === "applied" ||
          (st.status === "error" && Boolean(st.yaml));
        return (
          <div
            key={policy.id}
            className={`rounded-xl border p-4 shadow-sm ${
              st.status === "error"
                ? "border-red-500 bg-red-50 dark:bg-red-950"
                : "border-slate-200 dark:border-slate-800"
            }`}
          >
            <div className="flex items-center justify-between">
              <span className="font-medium">{policy.title}</span>
              <StatusBadge status={st.status} />
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
                  className="inline-flex items-center gap-1.5 rounded-lg bg-slate-200 px-3 py-1 text-sm shadow-sm dark:bg-slate-800"
                >
                  <Undo2 size={14} />
                  Revert
                </button>
              ) : (
                <button
                  onClick={() => apply(policy)}
                  disabled={st.status === "applying"}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
                >
                  <Play size={14} />
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
              <div className="mt-3">
                <CodeBlock code={st.yaml} />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
