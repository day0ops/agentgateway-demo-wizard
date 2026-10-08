import type { Pillar } from "../lib/api";

// Only pillars with a Teaser are features being demonstrated - the intro/
// wrap-up pillars this renders alongside don't set one.
export function WelcomeFeatures({ pillars }: { pillars: Pillar[] }) {
  const features = pillars.filter((p) => p.teaser);
  if (features.length === 0) return null;

  return (
    <div className="grid grid-cols-1 gap-3 px-6 pb-6 sm:grid-cols-2">
      {features.map((pillar, i) => (
        <div
          key={pillar.id}
          className="flex gap-3 rounded-xl border border-slate-200 p-4 shadow-sm dark:border-slate-800"
        >
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-indigo-50 text-xs font-semibold text-indigo-700 dark:bg-indigo-950 dark:text-indigo-300">
            {i + 1}
          </span>
          <div>
            <div className="font-medium">{pillar.title}</div>
            <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">
              {pillar.teaser}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}
