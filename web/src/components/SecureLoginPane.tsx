import { useEffect, useState } from "react";
import { CheckCircle2, LogIn } from "lucide-react";

type SessionState =
  | { status: "checking" }
  | { status: "loggedOut" }
  | { status: "loggedIn"; identity: string };

// SecureLoginPane replaces the auth pillar's old two-identity LoginPane -
// agentgateway's own ext-auth now owns the entire login flow (redirect to
// Keycloak, callback, session). The session cookie is httpOnly, so the only
// way to know whether one exists is to ask the backend - GET /secure/session
// is reached only if ext-auth already let the request through.
export function SecureLoginPane({
  returnToStepId,
}: {
  returnToStepId: string;
}) {
  const [session, setSession] = useState<SessionState>({ status: "checking" });

  useEffect(() => {
    let cancelled = false;
    fetch("/secure/session", { redirect: "manual" })
      .then(async (res) => {
        if (cancelled) return;
        if (res.type === "opaqueredirect" || !res.ok) {
          setSession({ status: "loggedOut" });
          return;
        }
        const body = (await res.json()) as { identity: string };
        setSession({ status: "loggedIn", identity: body.identity });
      })
      .catch(() => {
        if (!cancelled) setSession({ status: "loggedOut" });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Log in
      </h2>
      <div className="flex items-center gap-3 rounded-xl border border-slate-200 p-4 shadow-sm dark:border-zinc-800">
        {session.status === "loggedIn" ? (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
            <CheckCircle2 size={12} />
            logged in as {session.identity}
          </span>
        ) : (
          <a
            href={`/secure/login?returnTo=${encodeURIComponent(returnToStepId)}`}
            className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm transition-transform duration-150 ease-out active:scale-[0.97] dark:bg-zinc-100 dark:text-zinc-900"
          >
            <LogIn size={14} />
            Log in
          </a>
        )}
        <a
          href="/secure/logout"
          className="inline-flex items-center gap-1.5 rounded-lg bg-slate-200 px-3 py-1 text-sm shadow-sm transition-transform duration-150 ease-out active:scale-[0.97] dark:bg-zinc-800"
        >
          Log out
        </a>
      </div>
    </div>
  );
}
