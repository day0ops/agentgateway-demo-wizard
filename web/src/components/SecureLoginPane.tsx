// SecureLoginPane replaces the auth pillar's old two-identity LoginPane -
// agentgateway's own ext-auth now owns the entire login flow (redirect to
// Keycloak, callback, session). There's nothing for the SPA to poll: the
// session cookie is httpOnly by design, and the real signal of "logged in
// or not" is simply whether the next OpsAgent call succeeds.
export function SecureLoginPane() {
  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Log in
      </h2>
      <div className="flex items-center gap-3 rounded-xl border border-slate-200 p-4 shadow-sm dark:border-zinc-800">
        <a
          href="/secure/login?returnTo=auth"
          className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm transition-transform duration-150 ease-out active:scale-[0.97] dark:bg-zinc-100 dark:text-zinc-900"
        >
          Log in
        </a>
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
