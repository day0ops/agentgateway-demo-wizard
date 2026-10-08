import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusBadge } from "./StatusBadge";

describe("StatusBadge", () => {
  it("renders the default label for each status", () => {
    const { rerender } = render(<StatusBadge status="applied" />);
    expect(screen.getByText("applied")).toBeInTheDocument();

    rerender(<StatusBadge status="applying" />);
    expect(screen.getByText("applying")).toBeInTheDocument();

    rerender(<StatusBadge status="error" />);
    expect(screen.getByText("error")).toBeInTheDocument();

    rerender(<StatusBadge status="idle" />);
    expect(screen.getByText("not applied")).toBeInTheDocument();
  });

  it("overrides the label when one is provided", () => {
    render(<StatusBadge status="applied" label="pre-provisioned" />);
    expect(screen.getByText("pre-provisioned")).toBeInTheDocument();
    expect(screen.queryByText("applied")).not.toBeInTheDocument();
  });
});
