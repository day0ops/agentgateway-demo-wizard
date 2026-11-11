import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToolModePane } from "./ToolModePane";
import * as api from "../lib/api";

const demo = {
  defaultPrompt: "q?",
  modes: [
    {
      key: "standard",
      label: "Standard",
      mcpPath: "/a",
      description: "All tool schemas, one call per turn.",
    },
    {
      key: "code",
      label: "Code",
      mcpPath: "/b",
      description: "Agent writes code the gateway runs.",
    },
  ],
};

describe("ToolModePane", () => {
  it("shows each mode's description", () => {
    render(<ToolModePane demo={demo} />);
    expect(
      screen.getByText("All tool schemas, one call per turn."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Agent writes code the gateway runs."),
    ).toBeInTheDocument();
  });

  it("shows savings vs the Standard baseline", async () => {
    vi.spyOn(api, "runToolMode").mockImplementation(async (mode) => ({
      answer: "ans",
      promptTokens: mode === "standard" ? 4000 : 1000,
      completionTokens: 100,
      hops: mode === "standard" ? 4 : 2,
      toolCalls: 1,
      toolsExposed: [],
      toolInvocations: [],
    }));

    render(<ToolModePane demo={demo} />);
    fireEvent.click(screen.getByRole("button", { name: /run comparison/i }));

    await waitFor(() => expect(screen.getByText(/%/)).toBeInTheDocument());
    // baseline row shows a dash, not a percentage
    expect(screen.getByTestId("saved-standard").textContent).toContain("-");
    expect(screen.getByTestId("saved-code").textContent).toMatch(/7[0-9]%/); // ~76% fewer tokens
  });

  it("expands a row to show the tools exposed and what it called", async () => {
    vi.spyOn(api, "runToolMode").mockImplementation(async (mode) => ({
      answer: "ans",
      promptTokens: 1000,
      completionTokens: 100,
      hops: 2,
      toolCalls: 1,
      toolsExposed:
        mode === "code" ? ["run_code"] : ["get_tool", "invoke_tool"],
      toolInvocations:
        mode === "code"
          ? [{ name: "run_code", arguments: '{"code":"return 42;"}' }]
          : [],
    }));

    render(<ToolModePane demo={demo} />);
    fireEvent.click(screen.getByRole("button", { name: /run comparison/i }));
    await waitFor(() => expect(screen.getByText(/%/)).toBeInTheDocument());

    fireEvent.click(
      screen.getByRole("button", { name: /show what code did/i }),
    );
    // "Tools exposed: run_code" - exact match so it doesn't also match the
    // longer call-detail text below
    expect(screen.getByText("run_code")).toBeInTheDocument();
    // the call detail itself, with the model's raw arguments
    expect(screen.getByText(/run_code\(/)).toBeInTheDocument();
    expect(screen.getByText(/return 42;/)).toBeInTheDocument();
  });
});
