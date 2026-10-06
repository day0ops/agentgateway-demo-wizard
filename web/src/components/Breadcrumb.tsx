import type { Pillar } from "../lib/api";

type BreadcrumbProps = {
  pillars: Pillar[];
  activePillarIndex: number;
  activeStepIndex: number;
  onSelectPillar: (pillarIndex: number) => void;
};

export function Breadcrumb({
  pillars,
  activePillarIndex,
  activeStepIndex,
  onSelectPillar,
}: BreadcrumbProps) {
  const activePillar = pillars[activePillarIndex];
  const stepCount = activePillar?.steps.length ?? 0;

  return (
    <nav className="flex items-center justify-between border-b-2 border-indigo-200 bg-indigo-50/70 px-6 py-3 dark:border-indigo-900/60 dark:bg-slate-900/60">
      <ol className="flex items-center gap-2 text-sm">
        {pillars.map((pillar, index) => (
          <li key={pillar.id} className="flex items-center gap-2">
            {index > 0 && (
              <span className="text-indigo-300 dark:text-indigo-800">-</span>
            )}
            <button
              onClick={() => onSelectPillar(index)}
              className={
                index === activePillarIndex
                  ? "font-semibold text-indigo-700 dark:text-indigo-300"
                  : "text-slate-500 hover:text-indigo-600 dark:hover:text-indigo-300"
              }
            >
              {pillar.title}
            </button>
          </li>
        ))}
      </ol>
      {stepCount > 1 && (
        <span className="rounded-full bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:bg-indigo-950 dark:text-indigo-300">
          step {activeStepIndex + 1} of {stepCount}
        </span>
      )}
    </nav>
  );
}
