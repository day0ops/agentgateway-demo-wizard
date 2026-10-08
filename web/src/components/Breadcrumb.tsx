import {
  Sparkles,
  Route,
  Shield,
  ShieldAlert,
  Eye,
  Bot,
  Flag,
  type LucideIcon,
} from "lucide-react";
import type { Pillar } from "../lib/api";

const PILLAR_ICONS: Record<string, LucideIcon> = {
  intro: Sparkles,
  routing: Route,
  auth: Shield,
  guardrails: ShieldAlert,
  cost: Eye,
  agent: Bot,
  wrap: Flag,
};

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
        {pillars.map((pillar, index) => {
          const Icon = PILLAR_ICONS[pillar.id];
          const isActive = index === activePillarIndex;
          return (
            <li key={pillar.id} className="flex items-center gap-2">
              {index > 0 && (
                <span className="text-indigo-300 dark:text-indigo-800">-</span>
              )}
              <button
                onClick={() => onSelectPillar(index)}
                className={
                  isActive
                    ? "inline-flex items-center gap-1.5 rounded-full bg-white px-2.5 py-1 font-semibold text-indigo-700 shadow-sm dark:bg-slate-800 dark:text-indigo-300"
                    : "inline-flex items-center gap-1.5 text-slate-500 hover:text-indigo-600 dark:hover:text-indigo-300"
                }
              >
                {Icon && <Icon size={14} />}
                {pillar.title}
              </button>
            </li>
          );
        })}
      </ol>
      {stepCount > 1 && (
        <div className="flex items-center gap-3">
          <div data-testid="step-segments" className="flex gap-1.5">
            {Array.from({ length: stepCount }, (_, i) => (
              <div
                key={i}
                className={`h-1.5 w-6 rounded-full transition-colors ${
                  i <= activeStepIndex
                    ? "bg-indigo-600 dark:bg-indigo-400"
                    : "bg-indigo-100 dark:bg-indigo-950"
                }`}
              />
            ))}
          </div>
          <span className="rounded-full bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700 dark:bg-indigo-950 dark:text-indigo-300">
            step {activeStepIndex + 1} of {stepCount}
          </span>
        </div>
      )}
    </nav>
  );
}
