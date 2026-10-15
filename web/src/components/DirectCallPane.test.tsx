import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DirectCallPane } from "./DirectCallPane";
import type { DirectCallDemo } from "../lib/api";

const config: DirectCallDemo = {
  title: "Try a direct transfer (no delegation chain)",
  path: "/core-banking-delegation/mcp",
  tool: "transfer",
  arguments: { from_account: "savings", to_account: "checking", amount: 500 },
};

describe("DirectCallPane", () => {
  it("posts the configured path/tool/arguments and renders the result", async () => {
    let sentBody: unknown;
    globalThis.fetch = ((_url: RequestInfo | URL, init?: RequestInit) => {
      sentBody = JSON.parse(init!.body as string);
      return Promise.resolve({
        ok: true,
        json: async () => ({
          statusCode: 200,
          body: '{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"denied"}],"isError":true}}',
          latencyMs: 12,
        }),
      } as unknown as Response);
    }) as unknown as typeof fetch;

    render(<DirectCallPane config={config} />);
    fireEvent.click(screen.getByText(config.title));

    await waitFor(() => expect(screen.getByText(/denied/)).toBeInTheDocument());
    expect(sentBody).toEqual({
      path: "/core-banking-delegation/mcp",
      tool: "transfer",
      arguments: {
        from_account: "savings",
        to_account: "checking",
        amount: 500,
      },
    });
  });

  it("surfaces a non-OK response as an error", async () => {
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: false,
        status: 401,
        text: async () => "log in first - see the login step above",
      } as unknown as Response)) as unknown as typeof fetch;

    render(<DirectCallPane config={config} />);
    fireEvent.click(screen.getByText(config.title));

    await waitFor(() =>
      expect(
        screen.getByText("log in first - see the login step above"),
      ).toBeInTheDocument(),
    );
  });
});
