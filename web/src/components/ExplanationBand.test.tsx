import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import mermaid from "mermaid";
import type { Step } from "../lib/api";
import { setTheme } from "../lib/theme";
import { ExplanationBand } from "./ExplanationBand";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn().mockResolvedValue({ svg: "<svg></svg>" }),
  },
}));

const step: Step = {
  id: "fallback",
  title: "Automatic fallback",
  explanation: "Watch the gateway fail over automatically.",
  diagram: "sequenceDiagram\n  A->>B: hi",
  policies: [],
  presets: [],
  agentDemo: false,
  virtualKeys: false,
  loginDemo: false,
};

describe("ExplanationBand", () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
  });

  it("renders the step title and explanation text", () => {
    render(<ExplanationBand step={step} />);
    expect(screen.getByText("Automatic fallback")).toBeInTheDocument();
    expect(
      screen.getByText("Watch the gateway fail over automatically."),
    ).toBeInTheDocument();
  });

  it("hides the diagram by default and reveals it on toggle", async () => {
    render(<ExplanationBand step={step} />);

    expect(mermaid.render).not.toHaveBeenCalled();
    expect(screen.getByText("Show diagram")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Show diagram"));

    await waitFor(() => expect(mermaid.render).toHaveBeenCalled());
    expect(screen.getByText("Hide diagram")).toBeInTheDocument();
  });

  it("re-renders the diagram in the new color scheme when the theme toggles", async () => {
    render(<ExplanationBand step={step} />);
    fireEvent.click(screen.getByText("Show diagram"));

    await waitFor(() =>
      expect(mermaid.initialize).toHaveBeenLastCalledWith(
        expect.objectContaining({ theme: "default" }),
      ),
    );

    act(() => {
      setTheme("dark");
    });

    await waitFor(() =>
      expect(mermaid.initialize).toHaveBeenLastCalledWith(
        expect.objectContaining({ theme: "dark" }),
      ),
    );
  });
});
