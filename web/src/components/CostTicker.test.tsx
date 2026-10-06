import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CostTicker } from "./CostTicker";

describe("CostTicker", () => {
  it("renders nothing when no requests have been recorded", () => {
    const { container } = render(
      <CostTicker totalCostUsd={0} requestCount={0} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the running total once a request has been recorded", () => {
    render(<CostTicker totalCostUsd={0.0023} requestCount={4} />);
    expect(screen.getByText(/\$0\.0023/)).toBeInTheDocument();
    expect(screen.getByText(/4 requests/)).toBeInTheDocument();
  });
});
