import { useEffect, useRef, useState } from "react";
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

  const scrollRef = useRef<HTMLOListElement>(null);
  const activeRef = useRef<HTMLButtonElement>(null);
  // Fade the scroll edges only when there's more to see on that side, so the
  // pillar list reads as scrollable without clipping a title mid-word.
  const [edges, setEdges] = useState({ left: false, right: false });

  useEffect(() => {
    activeRef.current?.scrollIntoView({ inline: "nearest", block: "nearest" });
  }, [activePillarIndex]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const update = () =>
      setEdges({
        left: el.scrollLeft > 4,
        right: el.scrollLeft + el.clientWidth < el.scrollWidth - 4,
      });
    update();
    el.addEventListener("scroll", update, { passive: true });
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => {
      el.removeEventListener("scroll", update);
      observer.disconnect();
    };
  }, [pillars.length]);

  return (
    <nav className="flex items-center gap-4 border-b border-border bg-card px-6 py-2.5">
      <div className="relative min-w-0 flex-1">
        {/* Horizontal scroll rather than wrap, so long titles never push the
            step progress onto a second line. */}
        <ol
          ref={scrollRef}
          className="no-scrollbar flex items-center gap-0.5 overflow-x-auto"
        >
          {pillars.map((pillar, index) => {
            const Icon = PILLAR_ICONS[pillar.id];
            const isActive = index === activePillarIndex;
            return (
              <li key={pillar.id} className="flex shrink-0 items-center">
                {index > 0 && (
                  <span aria-hidden className="px-1 text-sm text-border-strong">
                    ·
                  </span>
                )}
                <button
                  ref={isActive ? activeRef : undefined}
                  onClick={() => onSelectPillar(index)}
                  aria-current={isActive ? "step" : undefined}
                  className={
                    isActive
                      ? "inline-flex items-center gap-1.5 whitespace-nowrap rounded-full bg-accent px-3 py-1 text-sm font-semibold text-accent-foreground"
                      : "inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-3 py-1 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                  }
                >
                  {Icon && <Icon size={14} className="shrink-0" />}
                  {pillar.title}
                </button>
              </li>
            );
          })}
        </ol>
        {edges.left && (
          <div className="pointer-events-none absolute inset-y-0 left-0 w-8 bg-gradient-to-r from-card to-transparent" />
        )}
        {edges.right && (
          <div className="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-card to-transparent" />
        )}
      </div>
      {stepCount > 1 && (
        <div className="flex shrink-0 items-center gap-3">
          <div data-testid="step-segments" className="flex gap-1">
            {Array.from({ length: stepCount }, (_, i) => (
              <div
                key={i}
                className={`h-1.5 w-5 rounded-full transition-colors ${
                  i <= activeStepIndex ? "bg-primary" : "bg-border-strong"
                }`}
              />
            ))}
          </div>
          <span className="whitespace-nowrap text-xs font-medium tabular-nums text-muted-foreground">
            step {activeStepIndex + 1} of {stepCount}
          </span>
        </div>
      )}
    </nav>
  );
}
