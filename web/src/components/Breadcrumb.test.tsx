import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Pillar } from "../lib/api";
import { Breadcrumb } from "./Breadcrumb";

const emptyStep = {
  explanation: "",
  diagram: "",
  policies: [],
  presets: [],
  agentDemo: false,
  virtualKeys: false,
};

const pillars: Pillar[] = [
  {
    id: "intro",
    title: "Welcome",
    steps: [{ id: "welcome", title: "Welcome", ...emptyStep }],
  },
  {
    id: "routing",
    title: "Routing",
    steps: [
      { id: "a", title: "A", ...emptyStep },
      { id: "b", title: "B", ...emptyStep },
    ],
  },
];

describe("Breadcrumb", () => {
  it("hides the step indicator for a single-step pillar", () => {
    render(
      <Breadcrumb
        pillars={pillars}
        activePillarIndex={0}
        activeStepIndex={0}
        onSelectPillar={() => {}}
      />,
    );
    expect(screen.queryByText(/step \d+ of \d+/)).not.toBeInTheDocument();
  });

  it("shows the step indicator for the active pillar", () => {
    render(
      <Breadcrumb
        pillars={pillars}
        activePillarIndex={1}
        activeStepIndex={0}
        onSelectPillar={() => {}}
      />,
    );
    expect(screen.getByText("step 1 of 2")).toBeInTheDocument();
  });

  it("calls onSelectPillar with the clicked pillar index", () => {
    const onSelectPillar = vi.fn();
    render(
      <Breadcrumb
        pillars={pillars}
        activePillarIndex={0}
        activeStepIndex={0}
        onSelectPillar={onSelectPillar}
      />,
    );
    fireEvent.click(screen.getByText("Routing"));
    expect(onSelectPillar).toHaveBeenCalledWith(1);
  });
});
