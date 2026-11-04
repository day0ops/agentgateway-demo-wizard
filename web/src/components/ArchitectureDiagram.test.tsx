import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ArchitectureDiagram } from "./ArchitectureDiagram";

describe("ArchitectureDiagram", () => {
  it("renders an accessible architecture image", () => {
    render(<ArchitectureDiagram />);

    const img = screen.getByRole("img", { name: /agentgateway/i });
    expect(img.tagName).toBe("CANVAS");
  });
});
