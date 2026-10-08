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
