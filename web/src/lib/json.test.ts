import { describe, expect, it } from "vitest";
import { extractErrorMessage, prettyPrintJson } from "./json";

describe("prettyPrintJson", () => {
  it("indents a minified JSON object", () => {
    expect(prettyPrintJson('{"a":1,"b":[2,3]}')).toBe(
      '{\n  "a": 1,\n  "b": [\n    2,\n    3\n  ]\n}',
    );
  });

  it("returns non-JSON text unchanged", () => {
    expect(prettyPrintJson("data: chunk1\n\n")).toBe("data: chunk1\n\n");
  });

  it("returns an empty string unchanged", () => {
    expect(prettyPrintJson("")).toBe("");
  });
});

describe("extractErrorMessage", () => {
  function fakeResponse(text: string): Response {
    return { text: async () => text } as unknown as Response;
  }

  it("extracts the error field from a JSON error body", async () => {
    const res = fakeResponse('{"error":"log in first"}');
    expect(await extractErrorMessage(res)).toBe("log in first");
  });

  it("returns plain text unchanged when the body isn't JSON", async () => {
    const res = fakeResponse("invalid request body");
    expect(await extractErrorMessage(res)).toBe("invalid request body");
  });

  it("falls back to the raw text when JSON has no error field", async () => {
    const res = fakeResponse('{"other":"value"}');
    expect(await extractErrorMessage(res)).toBe('{"other":"value"}');
  });
});
