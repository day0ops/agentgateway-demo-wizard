import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ArchitectureDiagram } from "./ArchitectureDiagram";

describe("ArchitectureDiagram", () => {
  it("renders the architecture diagram image", () => {
    render(<ArchitectureDiagram />);

    const img = screen.getByAltText(/agentgateway/i);
    expect(img).toHaveAttribute(
      "src",
      "/diagrams/agents-mcp-architecture.svg",
    );
  });
});
