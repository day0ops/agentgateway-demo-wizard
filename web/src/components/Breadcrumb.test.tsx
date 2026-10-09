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
  loginDemo: false,
};

const pillars: Pillar[] = [
  {
    id: "intro",
    title: "Welcome",
    teaser: "",
    steps: [{ id: "welcome", title: "Welcome", ...emptyStep }],
  },
  {
    id: "routing",
    title: "Routing",
    teaser: "",
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

  it("renders one progress segment per step in the active pillar", () => {
    render(
      <Breadcrumb
        pillars={pillars}
        activePillarIndex={1}
        activeStepIndex={0}
        onSelectPillar={() => {}}
      />,
    );
    const segments = screen.getByTestId("step-segments").children;
    expect(segments).toHaveLength(2);
  });

  it("resets progress segments when the active pillar changes", () => {
    const { rerender } = render(
      <Breadcrumb
        pillars={pillars}
        activePillarIndex={1}
        activeStepIndex={1}
        onSelectPillar={() => {}}
      />,
    );
    const segments = screen.getByTestId("step-segments").children;
    expect(segments[0].className).toContain("bg-indigo-600");
    expect(segments[1].className).toContain("bg-indigo-600");

    rerender(
      <Breadcrumb
        pillars={pillars}
        activePillarIndex={0}
        activeStepIndex={0}
        onSelectPillar={() => {}}
      />,
    );
    expect(screen.queryByTestId("step-segments")).not.toBeInTheDocument();
  });
});
