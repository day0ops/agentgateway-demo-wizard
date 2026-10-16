import { describe, expect, it } from "vitest";
import { interpretToolCall } from "./mcp";

const sse = (payload: object) =>
  `event: message\ndata: ${JSON.stringify(payload)}\n\n`;

describe("interpretToolCall", () => {
  it("reads a successful tool result's text", () => {
    const body = sse({
      jsonrpc: "2.0",
      id: 2,
      result: {
        content: [{ type: "text", text: "transfer settled: txn-001" }],
        isError: false,
      },
    });
    expect(interpretToolCall(200, body)).toEqual({
      kind: "success",
      text: "transfer settled: txn-001",
    });
  });

  it("treats a hidden-tool JSON-RPC error as a policy denial", () => {
    // agentgateway hides an unauthorized MCP tool, so a denied call surfaces as
    // this error rather than an HTTP 403.
    const body = sse({
      jsonrpc: "2.0",
      id: 2,
      error: { code: -32602, message: "Unknown tool: transfer" },
    });
    const out = interpretToolCall(200, body);
    if (out.kind !== "denied")
      throw new Error(`expected denied, got ${out.kind}`);
    expect(out.detail).toContain("Unknown tool: transfer");
  });

  it("treats an HTTP 403 as a denial regardless of body", () => {
    expect(interpretToolCall(403, "").kind).toBe("denied");
  });

  it("surfaces a genuine error as an error, not a denial", () => {
    const body = sse({
      jsonrpc: "2.0",
      id: 2,
      error: { code: -32603, message: "internal server error" },
    });
    expect(interpretToolCall(200, body)).toEqual({
      kind: "error",
      detail: "internal server error",
    });
  });

  it("parses plain (non-SSE) JSON bodies too", () => {
    const body = JSON.stringify({
      jsonrpc: "2.0",
      id: 2,
      result: { content: [], isError: false },
    });
    expect(interpretToolCall(200, body).kind).toBe("success");
  });

  it("falls back to an error for an unparseable body", () => {
    expect(interpretToolCall(502, "upstream exploded").kind).toBe("error");
  });
});
