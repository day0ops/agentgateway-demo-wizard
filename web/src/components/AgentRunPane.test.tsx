import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AgentRunPane } from "./AgentRunPane";

function mockStreamingFetch(events: string[]) {
  let i = 0;
  const reader = {
    read: async () => {
      if (i >= events.length) return { done: true, value: undefined };
      const value = new TextEncoder().encode(events[i]);
      i += 1;
      return { done: false, value };
    },
  };
  return (() =>
    Promise.resolve({
      ok: true,
      body: { getReader: () => reader },
    } as unknown as Response)) as unknown as typeof fetch;
}

describe("AgentRunPane", () => {
  it("streams hops and renders each one as it arrives", async () => {
    globalThis.fetch = mockStreamingFetch([
      'data: {"type":"model_call","message":"Calling the model (hop 1)..."}\n\n',
      'data: {"type":"final","message":"AAPL is trading at $150."}\n\n',
    ]);

    render(<AgentRunPane identity="team-alpha" model="gpt-4o-mini" />);
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText("AAPL is trading at $150.")).toBeInTheDocument(),
    );
    expect(
      screen.getByText("Calling the model (hop 1)..."),
    ).toBeInTheDocument();
  });

  it("surfaces a stream read failure instead of leaving 'Run agent' stuck disabled", async () => {
    const reader = {
      read: async () => {
        throw new Error("stream disconnected");
      },
    };
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: true,
        body: { getReader: () => reader },
      } as unknown as Response)) as unknown as typeof fetch;

    render(<AgentRunPane identity="team-alpha" model="gpt-4o-mini" />);
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText("stream disconnected")).toBeInTheDocument(),
    );
    expect(screen.getByText("Run agent")).not.toBeDisabled();
  });

  it("surfaces a non-OK response as an error hop instead of reading the body as a stream", async () => {
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: false,
        status: 502,
        text: async () => "agent run failed: budget exceeded",
      } as unknown as Response)) as unknown as typeof fetch;

    render(<AgentRunPane identity="team-alpha" model="gpt-4o-mini" />);
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(
        screen.getByText("agent run failed: budget exceeded"),
      ).toBeInTheDocument(),
    );
  });
});
