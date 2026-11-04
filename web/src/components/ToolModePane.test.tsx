import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToolModePane } from "./ToolModePane";
import * as api from "../lib/api";

const demo = {
  defaultPrompt: "q?",
  modes: [
    { key: "standard", label: "Standard", mcpPath: "/a", description: "" },
    { key: "code", label: "Code", mcpPath: "/b", description: "" },
  ],
};

describe("ToolModePane", () => {
  it("shows savings vs the Standard baseline", async () => {
    vi.spyOn(api, "runToolMode").mockImplementation(async (mode) => ({
      answer: "ans",
      promptTokens: mode === "standard" ? 4000 : 1000,
      completionTokens: 100,
      hops: mode === "standard" ? 4 : 2,
      toolCalls: 1,
    }));

    render(<ToolModePane demo={demo} />);
    fireEvent.click(screen.getByRole("button", { name: /run comparison/i }));

    await waitFor(() => expect(screen.getByText(/%/)).toBeInTheDocument());
    // baseline row shows a dash, not a percentage
    expect(screen.getByTestId("saved-standard").textContent).toContain("-");
    expect(screen.getByTestId("saved-code").textContent).toMatch(/7[0-9]%/); // ~76% fewer tokens
  });
});
