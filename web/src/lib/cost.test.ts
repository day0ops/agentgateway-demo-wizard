import { describe, expect, it } from "vitest";
import { estimateCostFromResponseBody } from "./cost";

describe("estimateCostFromResponseBody", () => {
  it("computes cost from a known model's usage block", () => {
    const body = JSON.stringify({
      model: "gpt-4o-mini",
      usage: { prompt_tokens: 1000, completion_tokens: 1000 },
    });
    const result = estimateCostFromResponseBody(body);
    expect(result).not.toBeNull();
    expect(result!.costUsd).toBeCloseTo(0.00015 + 0.0006, 6);
    expect(result!.model).toBe("gpt-4o-mini");
  });

  it("returns null for a body without a usage block", () => {
    const body = JSON.stringify({ error: "blocked" });
    expect(estimateCostFromResponseBody(body)).toBeNull();
  });

  it("returns null for an unpriced model", () => {
    const body = JSON.stringify({
      model: "claude-sonnet-4-5-20250415",
      usage: { prompt_tokens: 100, completion_tokens: 100 },
    });
    expect(estimateCostFromResponseBody(body)).toBeNull();
  });

  it("returns null for unparseable JSON", () => {
    expect(estimateCostFromResponseBody("not json")).toBeNull();
  });
});
