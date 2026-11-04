import type { Pillar } from "../lib/api";

// Only pillars with a Teaser are features being demonstrated - the intro/
// wrap-up pillars this renders alongside don't set one.
export function WelcomeFeatures({ pillars }: { pillars: Pillar[] }) {
  const features = pillars.filter((p) => p.teaser);
  if (features.length === 0) return null;

  return (
    <div className="grid grid-cols-1 gap-x-10 gap-y-px px-6 pb-8 pt-4 sm:grid-cols-2">
      {features.map((pillar, i) => (
        <article
          key={pillar.id}
          className="group flex items-baseline gap-4 border-t border-border py-4 transition-colors hover:border-border-strong"
        >
          <span className="font-mono text-sm font-semibold tabular-nums text-primary">
            {String(i + 1).padStart(2, "0")}
          </span>
          <div className="min-w-0">
            <div className="font-medium text-foreground">{pillar.title}</div>
            <p className="mt-1 text-sm leading-relaxed text-muted-foreground text-pretty">
              {pillar.teaser}
            </p>
          </div>
        </article>
      ))}
    </div>
  );
}
