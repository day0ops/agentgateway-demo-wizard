import { useState } from "react";
import type { ToolModeDemo, ToolModeResult } from "../lib/api";
import { runToolMode } from "../lib/api";
import { costFromTokens } from "../lib/cost";
import { PaneLabel } from "./PaneLabel";
import { btnPrimary, inputBase } from "../lib/ui";

type Row =
  | { status: "idle" }
  | { status: "running" }
  | { status: "done"; result: ToolModeResult }
  | { status: "error"; message: string };

const MODEL = "gpt-4o-mini";

export function ToolModePane({ demo }: { demo: ToolModeDemo }) {
  const [prompt, setPrompt] = useState(demo.defaultPrompt);
  const [rows, setRows] = useState<Record<string, Row>>({});
  const [running, setRunning] = useState(false);

  // Found by key, not position: a future reorder of demo.modes must not
  // silently change which row the savings are computed against.
  const baselineMode =
    demo.modes.find((m) => m.key === "standard") ?? demo.modes[0];

  const baseline = (() => {
    const r = rows[baselineMode.key];
    return r && r.status === "done" ? r.result : null;
  })();

  async function run() {
    setRunning(true);
    setRows(
      Object.fromEntries(demo.modes.map((m) => [m.key, { status: "running" }])),
    );
    // Standard first so later rows can show savings against it.
    for (const mode of demo.modes) {
      try {
        const result = await runToolMode(mode.key, prompt);
        setRows((r) => ({ ...r, [mode.key]: { status: "done", result } }));
      } catch (err) {
        setRows((r) => ({
          ...r,
          [mode.key]: {
            status: "error",
            message: err instanceof Error ? err.message : String(err),
          },
        }));
      }
    }
    setRunning(false);
  }

  function tokens(res: ToolModeResult) {
    return res.promptTokens + res.completionTokens;
  }

  function saved(key: string, res: ToolModeResult): string {
    if (!baseline || key === baselineMode.key) return "-";
    const base = tokens(baseline);
    if (base === 0) return "-";
    const pct = Math.round((1 - tokens(res) / base) * 100);
    return `${pct}%`;
  }

  return (
    <div className="space-y-4">
      <PaneLabel>Tool modes</PaneLabel>
      <div className="space-y-3 rounded-xl border border-border bg-card p-4 shadow-card">
        <textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          rows={2}
          className={`${inputBase} w-full resize-y`}
        />
        <button onClick={run} disabled={running} className={btnPrimary}>
          Run comparison
        </button>

        <div className="overflow-hidden rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="bg-elevated text-left text-xs text-subtle">
              <tr>
                <th className="px-3 py-2 font-medium">Mode</th>
                <th className="px-3 py-2 font-medium tabular-nums">
                  Round-trips
                </th>
                <th className="px-3 py-2 font-medium tabular-nums">Tokens</th>
                <th className="px-3 py-2 font-medium tabular-nums">Est. $</th>
                <th className="px-3 py-2 font-medium tabular-nums">Saved</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {demo.modes.map((m) => {
                const row = rows[m.key] ?? { status: "idle" };
                return (
                  <tr key={m.key}>
                    <td className="px-3 py-2 font-medium text-foreground">
                      {m.label}
                    </td>
                    {row.status === "done" ? (
                      <>
                        <td className="px-3 py-2 tabular-nums text-muted-foreground">
                          {row.result.hops}
                        </td>
                        <td className="px-3 py-2 tabular-nums text-muted-foreground">
                          {tokens(row.result).toLocaleString()}
                        </td>
                        <td className="px-3 py-2 tabular-nums text-muted-foreground">
                          $
                          {(
                            costFromTokens(
                              MODEL,
                              row.result.promptTokens,
                              row.result.completionTokens,
                            ) ?? 0
                          ).toFixed(4)}
                        </td>
                        <td
                          data-testid={`saved-${m.key}`}
                          className="px-3 py-2 font-medium tabular-nums text-primary"
                        >
                          {saved(m.key, row.result)}
                        </td>
                      </>
                    ) : row.status === "error" ? (
                      <td
                        colSpan={4}
                        className="px-3 py-2 text-sm text-destructive"
                      >
                        {row.message}
                      </td>
                    ) : (
                      <td colSpan={4} className="px-3 py-2 text-xs text-subtle">
                        {row.status === "running" ? "running..." : "not run"}
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        {baseline && (
          <p className="text-sm leading-relaxed text-muted-foreground">
            Same answer every time: {baseline.answer}
          </p>
        )}
      </div>
    </div>
  );
}
