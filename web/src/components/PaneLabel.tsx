import type { ReactNode } from "react";

// The small section eyebrow shared by every pane (Configure, Request, Agent,
// ...). A restrained tracked-uppercase label with a teal tick, so the panes
// read as one system instead of each inventing its own heading.
export function PaneLabel({ children }: { children: ReactNode }) {
  return (
    <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.1em] text-subtle">
      <span aria-hidden className="h-3 w-0.5 rounded-full bg-primary/70" />
      {children}
    </h3>
  );
}
