// Interprets the raw MCP tools/call response that /secure/direct-call returns,
// so the UI can tell three things apart: the call succeeded, it was denied by a
// gateway policy, or it genuinely errored. agentgateway enforces MCP tool RBAC
// by hiding the tool (progressive disclosure), so a denied call comes back as a
// JSON-RPC "Unknown tool" error rather than an HTTP 403 - without this mapping
// the UI would just show that raw, confusing error.

export type DirectCallOutcome =
  | { kind: "success"; text: string }
  | { kind: "denied"; detail: string }
  | { kind: "error"; detail: string };

const DENIAL_PATTERN =
  /unknown tool|not authorized|unauthorized|forbidden|permission|denied|not permitted/i;

// The body may be SSE-framed ("event: message\ndata: {...}\n\n") or plain JSON.
function parseJsonRpc(body: string): Record<string, unknown> | null {
  const text = (body ?? "").trim();
  if (!text) return null;
  const dataLine = text.split("\n").find((line) => line.startsWith("data:"));
  const jsonStr = dataLine
    ? dataLine.slice(dataLine.indexOf(":") + 1).trim()
    : text;
  try {
    return JSON.parse(jsonStr);
  } catch {
    return null;
  }
}

function resultText(result: Record<string, unknown>): string {
  const content = result.content;
  if (Array.isArray(content)) {
    const joined = content
      .map((part) =>
        part && typeof part === "object" && "text" in part
          ? String(part.text ?? "")
          : "",
      )
      .filter(Boolean)
      .join("\n");
    if (joined) return joined;
  }
  return JSON.stringify(result);
}

export function interpretToolCall(
  statusCode: number,
  body: string,
): DirectCallOutcome {
  const payload = parseJsonRpc(body);

  const error = payload?.error as { message?: unknown } | undefined;
  if (error) {
    const detail = String(error.message ?? "the request was rejected");
    if (statusCode === 403 || DENIAL_PATTERN.test(detail))
      return { kind: "denied", detail };
    return { kind: "error", detail };
  }

  const result = payload?.result as Record<string, unknown> | undefined;
  if (result) {
    const text = resultText(result);
    if (result.isError) {
      return DENIAL_PATTERN.test(text)
        ? { kind: "denied", detail: text }
        : { kind: "error", detail: text || "the tool returned an error" };
    }
    return { kind: "success", text: text || "Completed." };
  }

  if (statusCode === 403) return { kind: "denied", detail: "forbidden" };
  if (statusCode >= 400) return { kind: "error", detail: `HTTP ${statusCode}` };
  return { kind: "error", detail: body || "no response from the gateway" };
}
