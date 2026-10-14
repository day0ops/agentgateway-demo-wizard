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
      <div className="divide-y divide-slate-200 rounded-xl border border-slate-200 shadow-sm dark:divide-zinc-800 dark:border-zinc-800">
        {identities.map((identity) => (
          <div key={identity} className="flex items-center justify-between gap-3 p-4">
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-brand-50 text-sm font-semibold text-brand-700 dark:bg-brand-950 dark:text-brand-300">
                {identity.charAt(0).toUpperCase()}
              </span>
              <div>
                <div className="font-mono text-sm font-medium">{identity}</div>
                <div className="text-xs text-slate-500">Demo customer identity</div>
              </div>
            </div>
            {status[identity] ? (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
                <CheckCircle2 size={12} />
                logged in (real token)
              </span>
            ) : (
              <a
                href={`/auth/login?identity=${identity}&return_to=auth`}
                className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm transition-transform duration-150 ease-out active:scale-[0.97] dark:bg-zinc-100 dark:text-zinc-900"
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
