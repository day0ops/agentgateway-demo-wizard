import { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Step } from "../lib/api";
import { getTheme, onThemeChange } from "../lib/theme";
import { markdownComponents } from "../lib/markdown";

// Tracks the color theme reactively, so the diagram effect below can depend
// on it directly instead of only re-reading the DOM once per step change.
function useCurrentTheme() {
  const [theme, setThemeState] = useState(getTheme);
  useEffect(() => onThemeChange(() => setThemeState(getTheme())), []);
  return theme;
}

// Mermaid themeVariables mapped from the design tokens (mermaid needs literal
// colors, not CSS vars). Covers the sequence-diagram roles the step diagrams
// use: actors, signals, and notes.
function mermaidThemeVariables(theme: "light" | "dark") {
  if (theme === "dark") {
    return {
      darkMode: true,
      background: "#10151c",
      primaryColor: "#161d27",
      primaryBorderColor: "#303b4a",
      primaryTextColor: "#e6ebf2",
      secondaryColor: "#1b232e",
      tertiaryColor: "#10151c",
      lineColor: "#64748b",
      textColor: "#e6ebf2",
      actorBkg: "#161d27",
      actorBorder: "#2dbcac",
      actorTextColor: "#e6ebf2",
      signalColor: "#94a3b8",
      signalTextColor: "#cbd5e1",
      labelBoxBkgColor: "#161d27",
      labelBoxBorderColor: "#303b4a",
      labelTextColor: "#e6ebf2",
      noteBkgColor: "#10211f",
      noteBorderColor: "#2dbcac",
      noteTextColor: "#cfeeea",
    };
  }
  return {
    background: "#ffffff",
    primaryColor: "#f1f5f9",
    primaryBorderColor: "#cbd5e1",
    primaryTextColor: "#0d1526",
    secondaryColor: "#e2e8f0",
    tertiaryColor: "#f8fafc",
    lineColor: "#64748b",
    textColor: "#0d1526",
    actorBkg: "#f1f5f9",
    actorBorder: "#0e9a8b",
    actorTextColor: "#0d1526",
    signalColor: "#5a677a",
    signalTextColor: "#334155",
    labelBoxBkgColor: "#f1f5f9",
    labelBoxBorderColor: "#cbd5e1",
    labelTextColor: "#0d1526",
    noteBkgColor: "#e0f4f1",
    noteBorderColor: "#0e9a8b",
    noteTextColor: "#0c6f64",
  };
}

export function ExplanationBand({ step }: { step: Step }) {
  const diagramRef = useRef<HTMLDivElement>(null);
  const theme = useCurrentTheme();
  const [showDiagram, setShowDiagram] = useState(false);

  useEffect(() => {
    if (!step.diagram || !diagramRef.current) return;

    // Drive mermaid from the design tokens so diagrams match the surrounding
    // surfaces in both themes, instead of its stock dark/default palettes.
    mermaid.initialize({
      startOnLoad: false,
      theme: "base",
      fontFamily: '"Geist Sans", ui-sans-serif, system-ui, sans-serif',
      themeVariables: mermaidThemeVariables(theme),
    });

    const id = `diagram-${step.id}`;
    mermaid.render(id, step.diagram).then(({ svg }) => {
      if (diagramRef.current) diagramRef.current.innerHTML = svg;
    });
  }, [step.id, step.diagram, theme, showDiagram]);

  return (
    <section className="border-b border-border px-6 py-7">
      <h2 className="text-2xl font-semibold tracking-tight text-foreground text-balance">
        {step.title}
      </h2>
      <div className="mt-3 max-w-[68ch] text-[15px] leading-7 text-muted-foreground">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={markdownComponents}
        >
          {step.explanation}
        </ReactMarkdown>
      </div>
      {step.diagram && (
        <>
          <button
            onClick={() => setShowDiagram((v) => !v)}
            className="mt-4 inline-flex items-center gap-1.5 rounded-lg border border-border bg-card px-3 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground active:scale-[0.97]"
          >
            <span
              className={`transition-transform ${showDiagram ? "rotate-90" : ""}`}
            >
              ›
            </span>
            {showDiagram ? "Hide diagram" : "Show diagram"}
          </button>
          {showDiagram && (
            <div ref={diagramRef} className="mt-4 flex justify-center" />
          )}
        </>
      )}
    </section>
  );
}
