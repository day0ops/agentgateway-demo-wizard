import { useState } from "react";

type CreatedKey = {
  name: string;
  key: string;
  userId: string;
};

type FormState = { name: string; userId: string; tokenBudget: string };
const emptyForm: FormState = { name: "", userId: "", tokenBudget: "5000" };

// The backend returns plain text for request-validation errors (400) and a
// {"error": "..."} JSON body for downstream failures (500) - parse the
// latter so the UI shows a clean sentence instead of a raw JSON blob.
async function extractErrorMessage(res: Response): Promise<string> {
  const text = await res.text();
  try {
    const parsed = JSON.parse(text) as { error?: string };
    return parsed.error ?? text;
  } catch {
    return text;
  }
}

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
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Virtual Keys
      </h2>

      <div className="space-y-3 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
        <div className="flex flex-wrap gap-2">
          <input
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Key name (e.g. team-gamma)"
            className="rounded border border-slate-200 bg-transparent px-2 py-1 text-sm dark:border-slate-700"
          />
          <input
            value={form.userId}
            onChange={(e) => setForm({ ...form, userId: e.target.value })}
            placeholder="User id"
            className="rounded border border-slate-200 bg-transparent px-2 py-1 text-sm dark:border-slate-700"
          />
          <input
            value={form.tokenBudget}
            onChange={(e) => setForm({ ...form, tokenBudget: e.target.value })}
            placeholder="Token budget"
            className="w-28 rounded border border-slate-200 bg-transparent px-2 py-1 text-sm dark:border-slate-700"
          />
          <button
            onClick={createKey}
            className="rounded bg-slate-900 px-3 py-1 text-sm text-white dark:bg-slate-100 dark:text-slate-900"
          >
            Create key
          </button>
        </div>

        {error && (
          <p className="text-sm text-red-700 dark:text-red-300">{error}</p>
        )}

        {keys.map((key) => (
          <div
            key={key.name}
            className="space-y-1 rounded border border-slate-200 p-3 text-sm dark:border-slate-800"
          >
            <div className="flex items-center justify-between">
              <span className="font-medium">
                {key.name} ({key.userId})
              </span>
              <div className="flex gap-2">
                <button
                  onClick={() => rotateKey(key.name)}
                  className="rounded bg-slate-200 px-2 py-0.5 text-xs dark:bg-slate-800"
                >
                  Rotate
                </button>
                <button
                  onClick={() => sendTestRequest(key)}
                  className="rounded bg-slate-200 px-2 py-0.5 text-xs dark:bg-slate-800"
                >
                  Send test request
                </button>
              </div>
            </div>
            <code className="block overflow-x-auto rounded bg-slate-950 p-2 text-xs text-slate-100">
              {key.key}
            </code>
            {testResult[key.name] && (
              <pre className="overflow-x-auto rounded bg-slate-950 p-2 text-xs text-slate-100">
                {testResult[key.name]}
              </pre>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
