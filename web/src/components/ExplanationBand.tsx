import { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";
import type { Step } from "../lib/api";
import { getTheme, onThemeChange } from "../lib/theme";

// Tracks the color theme reactively, so the diagram effect below can depend
// on it directly instead of only re-reading the DOM once per step change.
function useCurrentTheme() {
  const [theme, setThemeState] = useState(getTheme);
  useEffect(() => onThemeChange(() => setThemeState(getTheme())), []);
  return theme;
}

export function ExplanationBand({ step }: { step: Step }) {
  const diagramRef = useRef<HTMLDivElement>(null);
  const theme = useCurrentTheme();
  const [showDiagram, setShowDiagram] = useState(false);

  useEffect(() => {
    if (!step.diagram || !diagramRef.current) return;

    mermaid.initialize({
      startOnLoad: false,
      theme: theme === "dark" ? "dark" : "default",
    });

    const id = `diagram-${step.id}`;
    mermaid.render(id, step.diagram).then(({ svg }) => {
      if (diagramRef.current) diagramRef.current.innerHTML = svg;
    });
  }, [step.id, step.diagram, theme, showDiagram]);

  return (
    <section className="border-b border-slate-200 px-6 py-6 dark:border-zinc-800">
      <h2 className="text-lg font-semibold">{step.title}</h2>
      <p className="mt-2 max-w-3xl text-sm leading-relaxed text-slate-700 dark:text-zinc-300">
        {step.explanation}
      </p>
      {step.diagram && (
        <>
          <button
            onClick={() => setShowDiagram((v) => !v)}
            className="mt-3 inline-flex items-center gap-1.5 rounded-full border border-brand-200 bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700 shadow-sm transition-colors active:scale-[0.97] hover:bg-brand-100 dark:border-brand-900 dark:bg-brand-950 dark:text-brand-300 dark:hover:bg-brand-900"
          >
            <span
              className={`transition-transform ${showDiagram ? "rotate-90" : ""}`}
            >
              ›
            </span>
            {showDiagram ? "Hide diagram" : "Show diagram"}
          </button>
          {showDiagram && (
            <div ref={diagramRef} className="mt-3 flex justify-center" />
          )}
        </>
      )}
    </section>
  );
}
