import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Pillar } from "../lib/api";
import { WelcomeFeatures } from "./WelcomeFeatures";

const emptySteps: Pillar["steps"] = [];

describe("WelcomeFeatures", () => {
  it("renders nothing when no pillar has a teaser", () => {
    const pillars: Pillar[] = [
      { id: "intro", title: "Welcome", teaser: "", steps: emptySteps },
      { id: "wrap", title: "Wrap-up", teaser: "", steps: emptySteps },
    ];
    const { container } = render(<WelcomeFeatures pillars={pillars} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("lists only the pillars that have a teaser, in order", () => {
    const pillars: Pillar[] = [
      { id: "intro", title: "Welcome", teaser: "", steps: emptySteps },
      {
        id: "routing",
        title: "One endpoint, many models",
        teaser: "Smart model routing with automatic fallback.",
        steps: emptySteps,
      },
      {
        id: "auth",
        title: "Who's allowed to do what",
        teaser: "Identity-aware access control.",
        steps: emptySteps,
      },
      { id: "wrap", title: "Wrap-up", teaser: "", steps: emptySteps },
    ];
    render(<WelcomeFeatures pillars={pillars} />);

    expect(screen.getByText("One endpoint, many models")).toBeInTheDocument();
    expect(
      screen.getByText("Smart model routing with automatic fallback."),
    ).toBeInTheDocument();
    expect(screen.getByText("Who's allowed to do what")).toBeInTheDocument();
    expect(screen.queryByText("Welcome")).not.toBeInTheDocument();
    expect(screen.queryByText("Wrap-up")).not.toBeInTheDocument();
  });
});
