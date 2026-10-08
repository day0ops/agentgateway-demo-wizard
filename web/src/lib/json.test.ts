import { describe, expect, it } from "vitest";
import { prettyPrintJson } from "./json";

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
