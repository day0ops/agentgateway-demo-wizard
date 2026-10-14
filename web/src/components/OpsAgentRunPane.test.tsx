import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OpsAgentRunPane } from "./OpsAgentRunPane";

function mockJsonFetch(body: unknown, ok = true) {
  return (() =>
    Promise.resolve({
      ok,
      status: ok ? 200 : 502,
      json: async () => body,
      text: async () => JSON.stringify(body),
    } as unknown as Response)) as unknown as typeof fetch;
}

describe("OpsAgentRunPane", () => {
  it("renders the agent's answer after a run", async () => {
    globalThis.fetch = mockJsonFetch({ answer: "Checking balance: $4,210.55" });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="accounts"
        mode=""
        defaultPrompt="What's my balance?"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(
        screen.getByText("Checking balance: $4,210.55"),
      ).toBeInTheDocument(),
    );
  });

  it("renders markdown formatting (lists, bold, links) in the answer", async () => {
    globalThis.fetch = mockJsonFetch({
      answer:
        "1. **Everyday Checking** - [details](https://dashboard.stripe.com/test/balance)\n2. **Rainy Day Savings**",
    });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="accounts"
        mode=""
        defaultPrompt="What's my balance?"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText("Everyday Checking")).toBeInTheDocument(),
    );
    expect(screen.getByText("Everyday Checking").tagName).toBe("STRONG");
    const link = screen.getByRole("link", { name: "details" });
    expect(link).toHaveAttribute(
      "href",
      "https://dashboard.stripe.com/test/balance",
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });

  it("renders the decoded delegation chain when present", async () => {
    globalThis.fetch = mockJsonFetch({
      answer: "Transferred $500 from savings to checking.",
      extra: {
        chain: {
          sub: "team-alpha",
          act: "settlement-agent",
          act_act: "transfer-orchestrator-agent",
        },
      },
    });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="transfer-orchestrator"
        mode=""
        defaultPrompt="Move $500 from savings to checking"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText(/act: settlement-agent/)).toBeInTheDocument(),
    );
    expect(
      screen.getByText(/act.act: transfer-orchestrator-agent/),
    ).toBeInTheDocument();
  });

  it("surfaces a non-OK response as an error instead of crashing on json()", async () => {
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: false,
        status: 401,
        text: async () => 'log in as "team-alpha" first',
      } as unknown as Response)) as unknown as typeof fetch;

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="accounts"
        mode=""
        defaultPrompt="What's my balance?"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(
        screen.getByText('log in as "team-alpha" first'),
      ).toBeInTheDocument(),
    );
  });
});
