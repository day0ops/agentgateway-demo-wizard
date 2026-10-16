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

const mockDirectCall = (statusCode: number, body: string) => {
  let sentBody: unknown;
  globalThis.fetch = ((_url: RequestInfo | URL, init?: RequestInit) => {
    sentBody = JSON.parse(init!.body as string);
    return Promise.resolve({
      ok: true,
      json: async () => ({ statusCode, body, latencyMs: 12 }),
    } as unknown as Response);
  }) as unknown as typeof fetch;
  return () => sentBody;
};

const sse = (payload: object) =>
  `event: message\ndata: ${JSON.stringify(payload)}\n\n`;

describe("DirectCallPane", () => {
  it("posts the configured path/tool/arguments", async () => {
    const getSent = mockDirectCall(
      200,
      sse({
        jsonrpc: "2.0",
        id: 2,
        result: { content: [{ type: "text", text: "ok" }] },
      }),
    );

    render(<DirectCallPane config={config} />);
    fireEvent.click(screen.getByText(config.title));

    await waitFor(() =>
      expect(screen.getByText(/went through/)).toBeInTheDocument(),
    );
    expect(getSent()).toEqual({
      path: "/core-banking-delegation/mcp",
      tool: "transfer",
      arguments: {
        from_account: "savings",
        to_account: "checking",
        amount: 500,
      },
    });
  });

  it("shows a hidden-tool denial as a clean 'Denied by policy' message", async () => {
    mockDirectCall(
      200,
      sse({
        jsonrpc: "2.0",
        id: 2,
        error: { code: -32602, message: "Unknown tool: transfer" },
      }),
    );

    render(<DirectCallPane config={config} />);
    fireEvent.click(screen.getByText(config.title));

    await waitFor(() =>
      expect(screen.getByText("Denied by policy")).toBeInTheDocument(),
    );
    // the raw cause is still shown, but framed as a denial
    expect(screen.getByText("Unknown tool: transfer")).toBeInTheDocument();
  });

  it("shows a successful transfer as allowed", async () => {
    mockDirectCall(
      200,
      sse({
        jsonrpc: "2.0",
        id: 2,
        result: {
          content: [{ type: "text", text: "transfer settled: txn-001" }],
          isError: false,
        },
      }),
    );

    render(<DirectCallPane config={config} />);
    fireEvent.click(screen.getByText(config.title));

    await waitFor(() =>
      expect(
        screen.getByText("Allowed - the transfer went through"),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("transfer settled: txn-001")).toBeInTheDocument();
  });

  it("surfaces a non-OK HTTP response as an error", async () => {
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
