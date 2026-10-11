// Pretty-prints text as indented JSON if it parses as JSON, otherwise returns
// it unchanged - callers pass through arbitrary response bodies (SSE stream
// text, error strings) that aren't always a single JSON document.
export function prettyPrintJson(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

// The backend returns plain text for request-validation errors (400) and a
// {"error": "..."} JSON body for downstream failures (500) - parse the
// latter so the UI shows a clean sentence instead of a raw JSON blob.
export async function extractErrorMessage(res: Response): Promise<string> {
  const text = await res.text();
  try {
    const parsed = JSON.parse(text) as { error?: string };
    return parsed.error ?? text;
  } catch {
    return text;
  }
}
