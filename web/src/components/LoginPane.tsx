import { useEffect, useState } from "react";
import { LogIn, CheckCircle2 } from "lucide-react";

type LoginStatus = Record<string, boolean>;

// LoginPane replaces Configure/Request for the auth pillar's login step -
// there's no policy to apply and no request to drive, just a real
// Authorization Code redirect per identity. The session cookie set by
// /auth/callback is httpOnly, so status is read back from the server
// instead of from anything client-readable.
export function LoginPane({ identities }: { identities: string[] }) {
  const [status, setStatus] = useState<LoginStatus>({});

  useEffect(() => {
    identities.forEach((identity) => {
      fetch(`/api/auth/status?identity=${identity}`)
        .then((res) => res.json())
        .then((data: { loggedIn: boolean }) =>
          setStatus((s) => ({ ...s, [identity]: data.loggedIn })),
        )
        .catch(() => undefined);
    });
  }, [identities]);

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Log in
      </h2>
      <div className="space-y-3 rounded-xl border border-slate-200 p-4 shadow-sm dark:border-slate-800">
        {identities.map((identity) => (
          <div key={identity} className="flex items-center justify-between">
            <span className="font-mono text-sm">{identity}</span>
            {status[identity] ? (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
                <CheckCircle2 size={12} />
                logged in (real token)
              </span>
            ) : (
              <a
                href={`/auth/login?identity=${identity}&return_to=auth`}
                className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm dark:bg-slate-100 dark:text-slate-900"
              >
                <LogIn size={14} />
                Log in as {identity}
              </a>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
