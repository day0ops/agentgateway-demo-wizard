import { useEffect, useState } from "react";
import { CheckCircle2, LogIn } from "lucide-react";
import { PaneLabel } from "./PaneLabel";
import { btnPrimary, btnSecondary } from "../lib/ui";

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
      <PaneLabel>Log in</PaneLabel>
      <div className="flex items-center gap-3 rounded-xl border border-border bg-card p-4 shadow-card">
        {session.status === "loggedIn" ? (
          <span className="inline-flex items-center gap-1.5 rounded-full border border-success-border bg-success-soft px-3 py-1 text-sm font-medium text-success">
            <CheckCircle2 size={14} />
            logged in as {session.identity}
          </span>
        ) : (
          <a
            href={`/secure/login?returnTo=${encodeURIComponent(returnToStepId)}`}
            className={btnPrimary}
          >
            <LogIn size={14} />
            Log in
          </a>
        )}
        <a href="/secure/logout" className={btnSecondary}>
          Log out
        </a>
      </div>
    </div>
  );
}
