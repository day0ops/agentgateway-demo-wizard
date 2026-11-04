import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { extractErrorMessage } from "../lib/json";
import { PaneLabel } from "./PaneLabel";
import { btnChip, btnPrimary, inputBase } from "../lib/ui";

type CreatedKey = {
  name: string;
  key: string;
  userId: string;
};

type FormState = { name: string; userId: string; tokenBudget: string };
const emptyForm: FormState = { name: "", userId: "", tokenBudget: "5000" };

export function VirtualKeyPane() {
  const [form, setForm] = useState<FormState>(emptyForm);
  const [keys, setKeys] = useState<CreatedKey[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<Record<string, string>>({});

  async function createKey() {
    setError(null);
    const tokenBudget = Number(form.tokenBudget);
    if (!form.name || !form.userId || !tokenBudget || tokenBudget <= 0) {
      setError("Name, user id, and a positive token budget are required.");
      return;
    }

    const res = await fetch("/api/virtual-keys", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: form.name,
        userId: form.userId,
        tokenBudget,
      }),
    });
    if (!res.ok) {
      setError(await extractErrorMessage(res));
      return;
    }
    const created = (await res.json()) as CreatedKey;
    setKeys((k) => [...k, created]);
    setForm(emptyForm);
  }

  async function rotateKey(name: string) {
    setError(null);
    const res = await fetch(`/api/virtual-keys/${name}/rotate`, {
      method: "POST",
    });
    if (!res.ok) {
      setError(await extractErrorMessage(res));
      return;
    }
    const rotated = (await res.json()) as CreatedKey;
    setKeys((ks) =>
      ks.map((k) => (k.name === name ? { ...k, key: rotated.key } : k)),
    );
  }

  async function sendTestRequest(key: CreatedKey) {
    const res = await fetch("/api/request", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        identity: "anonymous",
        method: "POST",
        path: "/cost/budgets",
        headers: { Authorization: `Bearer ${key.key}` },
        body: `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}`,
      }),
    });
    const body = await res.text();
    setTestResult((r) => ({ ...r, [key.name]: body }));
  }

  return (
    <div className="space-y-4">
      <PaneLabel>Virtual keys</PaneLabel>

      <div className="space-y-3 rounded-xl border border-border bg-card p-4 shadow-card">
        <div className="flex flex-wrap gap-2">
          <input
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Key name (e.g. team-gamma)"
            className={inputBase}
          />
          <input
            value={form.userId}
            onChange={(e) => setForm({ ...form, userId: e.target.value })}
            placeholder="User id"
            className={inputBase}
          />
          <input
            value={form.tokenBudget}
            onChange={(e) => setForm({ ...form, tokenBudget: e.target.value })}
            placeholder="Token budget"
            className={`${inputBase} w-28`}
          />
          <button onClick={createKey} className={btnPrimary}>
            Create key
          </button>
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}

        {keys.map((key) => (
          <div
            key={key.name}
            className="space-y-2 rounded-lg border border-border bg-elevated p-3 text-sm"
          >
            <div className="flex items-center justify-between">
              <span className="font-medium text-foreground">
                {key.name} ({key.userId})
              </span>
              <div className="flex gap-2">
                <button
                  onClick={() => rotateKey(key.name)}
                  className={btnChip}
                >
                  <RefreshCw size={12} />
                  Rotate
                </button>
                <button
                  onClick={() => sendTestRequest(key)}
                  className={btnChip}
                >
                  Send test request
                </button>
              </div>
            </div>
            <code className="block overflow-x-auto rounded-md border border-border bg-card p-2 font-mono text-xs text-foreground">
              {key.key}
            </code>
            {testResult[key.name] && (
              <pre className="overflow-x-auto rounded-md border border-border bg-card p-2 font-mono text-xs text-foreground">
                {testResult[key.name]}
              </pre>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
