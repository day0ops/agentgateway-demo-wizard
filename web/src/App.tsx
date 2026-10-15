import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { fetchConfig, fetchScenarios, type Pillar } from "./lib/api";
import { initTheme } from "./lib/theme";
import { estimateCostFromResponseBody } from "./lib/cost";
import { Breadcrumb } from "./components/Breadcrumb";
import { ExplanationBand } from "./components/ExplanationBand";
import { ThemeToggle } from "./components/ThemeToggle";
import { CostTicker } from "./components/CostTicker";
import { ConfigurePane } from "./components/ConfigurePane";
import { DriveRequestPane } from "./components/DriveRequestPane";
import { AgentRunPane } from "./components/AgentRunPane";
import { OpsAgentRunPane } from "./components/OpsAgentRunPane";
import { VirtualKeyPane } from "./components/VirtualKeyPane";
import { WelcomeFeatures } from "./components/WelcomeFeatures";
import { ArchitectureDiagram } from "./components/ArchitectureDiagram";
import { SecureLoginPane } from "./components/SecureLoginPane";

export default function App() {
  const [pillars, setPillars] = useState<Pillar[]>([]);
  const [activePillarIndex, setActivePillarIndex] = useState(0);
  const [activeStepIndex, setActiveStepIndex] = useState(0);
  const [missingEnvVars, setMissingEnvVars] = useState<string[]>([]);
  const [version, setVersion] = useState("");
  const [scenariosError, setScenariosError] = useState<string | null>(null);
  const [totalCostUsd, setTotalCostUsd] = useState(0);
  const [costRequestCount, setCostRequestCount] = useState(0);

  function recordResponseCost(body: string) {
    const cost = estimateCostFromResponseBody(body);
    if (!cost) return;
    setTotalCostUsd((t) => t + cost.costUsd);
    setCostRequestCount((c) => c + 1);
  }

  useEffect(() => {
    initTheme();
    fetchScenarios()
      .then((loadedPillars) => {
        setPillars(loadedPillars);
        // Additive: a login redirect (see SecureLoginPane) sends the browser back
        // to "/?returnTo=<pillar id>" instead of the default welcome screen.
        const returnTo = new URLSearchParams(window.location.search).get(
          "returnTo",
        );
        if (!returnTo) return;
        const index = loadedPillars.findIndex((p) => p.id === returnTo);
        if (index >= 0) {
          setActivePillarIndex(index);
          setActiveStepIndex(0);
        }
      })
      .catch((err) =>
        setScenariosError(err instanceof Error ? err.message : String(err)),
      );
    fetchConfig()
      .then((config) => {
        setMissingEnvVars(config.missing);
        setVersion(config.version);
      })
      .catch((err) => console.error("failed to load config", err));
  }, []);

  // Additive: surfaced whenever env vars are missing, loaded or not, since it
  // never blocks the wizard from being usable.
  const missingEnvBanner = missingEnvVars.length > 0 && (
    <div className="border-y border-amber-300 bg-amber-50 px-6 py-2 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200">
      Missing environment variables: {missingEnvVars.join(", ")} - some demo
      steps will fail until these are set.
    </div>
  );

  if (pillars.length === 0) {
    return (
      <div className="min-h-screen bg-white text-slate-900 dark:bg-zinc-950 dark:text-zinc-100">
        <h1 className="p-6 text-xl font-semibold">Agentgateway Demo Console</h1>
        {missingEnvBanner}
        {scenariosError && (
          <p className="px-6 pb-6 text-sm text-red-700 dark:text-red-300">
            Failed to load the demo script: {scenariosError}
          </p>
        )}
      </div>
    );
  }

  const activePillar = pillars[activePillarIndex];
  const activeStep = activePillar.steps[activeStepIndex];
  const hasDemo =
    activeStep.agentDemo ||
    activeStep.loginDemo ||
    !!activeStep.opsAgent ||
    activeStep.policies.length > 0 ||
    activeStep.presets.length > 0;

  function selectPillar(index: number) {
    setActivePillarIndex(index);
    setActiveStepIndex(0);
  }

  function nextStep() {
    if (activeStepIndex < activePillar.steps.length - 1) {
      setActiveStepIndex(activeStepIndex + 1);
    } else if (activePillarIndex < pillars.length - 1) {
      setActivePillarIndex(activePillarIndex + 1);
      setActiveStepIndex(0);
    }
  }

  function previousStep() {
    if (activeStepIndex > 0) {
      setActiveStepIndex(activeStepIndex - 1);
    } else if (activePillarIndex > 0) {
      const previousPillar = pillars[activePillarIndex - 1];
      setActivePillarIndex(activePillarIndex - 1);
      setActiveStepIndex(previousPillar.steps.length - 1);
    }
  }

  return (
    <div className="flex h-screen flex-col bg-white text-slate-900 dark:bg-zinc-950 dark:text-zinc-100">
      <div className="shrink-0 shadow-sm">
        <header className="flex items-center justify-between bg-brand-50/70 px-6 py-3 dark:bg-zinc-900/60">
          <h1 className="text-xl font-bold tracking-tight text-brand-700 dark:text-brand-300">
            Agentgateway Demo Console
          </h1>
          <div className="flex items-center gap-3">
            <CostTicker
              totalCostUsd={totalCostUsd}
              requestCount={costRequestCount}
            />
            <ThemeToggle />
          </div>
        </header>
        {missingEnvBanner}
        <Breadcrumb
          pillars={pillars}
          activePillarIndex={activePillarIndex}
          activeStepIndex={activeStepIndex}
          onSelectPillar={selectPillar}
        />
      </div>

      <div className="flex-1 overflow-y-auto">
        <ExplanationBand key={activeStep.id} step={activeStep} />
        {activeStep.id === "welcome" && <WelcomeFeatures pillars={pillars} />}
        {activeStep.id === "welcome" && <ArchitectureDiagram />}
        {activeStep.loginDemo && (
          <div className="px-6 py-6">
            <SecureLoginPane key={`${activeStep.id}-login`} />
          </div>
        )}
        {hasDemo && !activeStep.loginDemo && (
          <div className="grid grid-cols-2 gap-6 px-6 py-6">
            <ConfigurePane
              key={`${activeStep.id}-configure`}
              policies={activeStep.policies}
            />
            {activeStep.opsAgent ? (
              <OpsAgentRunPane
                key={`${activeStep.id}-ops-agent`}
                agent={activeStep.opsAgent.agent}
                mode={activeStep.opsAgent.mode}
                defaultPrompt={activeStep.opsAgent.defaultPrompt}
              />
            ) : activeStep.agentDemo ? (
              <AgentRunPane
                key={`${activeStep.id}-agent`}
                identity="team-alpha"
                model="gpt-4o-mini"
              />
            ) : (
              <DriveRequestPane
                key={`${activeStep.id}-request`}
                presets={activeStep.presets}
                onResponse={recordResponseCost}
              />
            )}
          </div>
        )}
        {activeStep.virtualKeys && (
          <div className="px-6 pb-6">
            <VirtualKeyPane />
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center justify-between border-t border-slate-200 bg-white px-6 py-3 dark:border-zinc-800 dark:bg-zinc-950">
        <button
          onClick={previousStep}
          className="inline-flex items-center gap-1 text-sm text-slate-500 transition-colors hover:text-brand-600 hover:underline dark:hover:text-brand-400"
        >
          <ChevronLeft size={14} />
          previous
        </button>
        {version && (
          <span className="text-xs text-slate-400 dark:text-zinc-600">
            {version}
          </span>
        )}
        <button
          onClick={nextStep}
          className="inline-flex items-center gap-1 text-sm text-slate-500 transition-colors hover:text-brand-600 hover:underline dark:hover:text-brand-400"
        >
          next
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  );
}
