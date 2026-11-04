import { useState } from "react";
import { Play, Undo2, Eye, EyeOff } from "lucide-react";
import type { Policy } from "../lib/api";
import { StatusBadge } from "./StatusBadge";
import { CodeBlock } from "./CodeBlock";
import { PaneLabel } from "./PaneLabel";
import { btnChip, btnPrimary, btnSecondary } from "../lib/ui";

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

  async function toggleYaml(policy: Policy) {
    const current = state[policy.id] ?? idleState;
    if (current.showYaml) {
      setState((s) => ({ ...s, [policy.id]: { ...current, showYaml: false } }));
      return;
    }
    if (current.yaml) {
      setState((s) => ({ ...s, [policy.id]: { ...current, showYaml: true } }));
      return;
    }

    const res = await fetch(`/api/config/view?policyId=${policy.id}`);
    if (!res.ok) {
      const message = await res.text();
      setState((s) => ({
        ...s,
        [policy.id]: {
          ...(s[policy.id] ?? current),
          status: "error",
          message,
          showYaml: false,
        },
      }));
      return;
    }
    const { refs } = (await res.json()) as {
      refs: { name: string; found: boolean; yaml?: string }[];
    };
    const yaml = refs
      .map((r) => r.yaml ?? "")
      .filter(Boolean)
      .join("\n---\n");
    setState((s) => ({
      ...s,
      [policy.id]: { ...(s[policy.id] ?? current), yaml, showYaml: true },
    }));
  }

  return (
    <div className="space-y-4">
      <PaneLabel>Configure</PaneLabel>
      {policies.map((policy) => {
        if (policy.readOnly) {
          const vst = viewState[policy.id] ?? idleViewState;
          return (
            <div
              key={policy.id}
              className="rounded-xl border border-border bg-card p-4 shadow-card"
            >
              <div className="flex items-center justify-between gap-2">
                {vst.status === "shown" ? (
                  <button
                    onClick={() => hideConfig(policy.id)}
                    className={btnSecondary}
                  >
                    <EyeOff size={14} />
                    Hide config
                  </button>
                ) : (
                  <button
                    onClick={() => viewConfig(policy)}
                    disabled={vst.status === "loading"}
                    className={btnPrimary}
                  >
                    <Eye size={14} />
                    View config
                  </button>
                )}
                <span className="text-xs text-subtle">pre-provisioned</span>
              </div>

              {vst.status === "error" && (
                <p className="mt-2 text-sm text-destructive">{vst.message}</p>
              )}
              {vst.status === "not-provisioned" && (
                <p className="mt-2 text-sm text-warning">
                  Not yet provisioned - deploy the field-kit usecase for this
                  demo first.
                </p>
              )}

              {vst.status === "shown" &&
                vst.refs
                  ?.filter((r) => r.found)
                  .map((r) => (
                    <div
                      key={r.name}
                      className="mt-3 overflow-hidden rounded-lg border border-border"
                    >
                      <div className="border-b border-border bg-elevated px-3 py-1.5 font-mono text-xs text-muted-foreground">
                        {r.name}
                      </div>
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
            className={`rounded-xl border p-4 shadow-card ${
              st.status === "error"
                ? "border-destructive bg-destructive-soft"
                : "border-border bg-card"
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <div className="flex gap-2">
                {canRevert ? (
                  <button
                    onClick={() => revert(policy)}
                    className={btnSecondary}
                  >
                    <Undo2 size={14} />
                    Revert
                  </button>
                ) : (
                  <button
                    onClick={() => apply(policy)}
                    disabled={st.status === "applying"}
                    className={btnPrimary}
                  >
                    <Play size={14} />
                    Apply
                  </button>
                )}
                <button onClick={() => toggleYaml(policy)} className={btnChip}>
                  <span
                    className={`transition-transform ${st.showYaml ? "rotate-90" : ""}`}
                  >
                    ›
                  </span>
                  {st.showYaml ? "hide YAML" : "view YAML"}
                </button>
              </div>
              <StatusBadge status={st.status} />
            </div>

            {st.status === "error" && (
              <p className="mt-2 text-sm text-destructive">{st.message}</p>
            )}

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
