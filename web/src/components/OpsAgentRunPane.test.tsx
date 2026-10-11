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
    globalThis.fetch = mockJsonFetch({ answer: "3 repos found" });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="repo"
        mode="standard"
        defaultPrompt="List our repos"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText("3 repos found")).toBeInTheDocument(),
    );
  });

  it("renders markdown formatting (lists, bold, links) in the answer", async () => {
    globalThis.fetch = mockJsonFetch({
      answer:
        "1. **payments-service** - [repo](https://github.com/day0ops/payments-service)\n2. **checkout-web**",
    });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="repo"
        mode="standard"
        defaultPrompt="List our repos"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText("payments-service")).toBeInTheDocument(),
    );
    expect(screen.getByText("payments-service").tagName).toBe("STRONG");
    const link = screen.getByRole("link", { name: "repo" });
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/day0ops/payments-service",
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });

  it("renders the decoded delegation chain when present", async () => {
    globalThis.fetch = mockJsonFetch({
      answer: "Deployed payments-service.",
      extra: {
        chain: {
          sub: "team-alpha",
          act: "deploy-agent",
          act_act: "release-manager-agent",
        },
      },
    });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="release-manager"
        mode=""
        defaultPrompt="Deploy payments-service"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText(/act: deploy-agent/)).toBeInTheDocument(),
    );
    expect(
      screen.getByText(/act.act: release-manager-agent/),
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
        agent="repo"
        mode="standard"
        defaultPrompt="List our repos"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(
        screen.getByText('log in as "team-alpha" first'),
      ).toBeInTheDocument(),
    );
  });

  it("shows a clean message instead of a raw JSON blob when the backend returns a JSON error", async () => {
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: false,
        status: 401,
        text: async () =>
          '{"error":"log in as \\"team-alpha\\" first - see the login step above"}',
      } as unknown as Response)) as unknown as typeof fetch;

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="repo"
        mode="standard"
        defaultPrompt="List our repos"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(
        screen.getByText(
          'log in as "team-alpha" first - see the login step above',
        ),
      ).toBeInTheDocument(),
    );
  });
});
