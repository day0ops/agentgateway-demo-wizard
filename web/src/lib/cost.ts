// Per-1M-token USD rates, mirroring the cost-attribution step's own model
// cost catalog (internal/manifests/templates/cost/attribution.yaml.tmpl).
// gpt-4o-mini only, since that's the only model currently priced there.
const RATES_PER_MILLION_TOKENS: Record<
  string,
  { input: number; output: number }
> = {
  "gpt-4o-mini": { input: 0.15, output: 0.6 },
};

export type UsageCost = { costUsd: number; model: string } | null;

// estimateCostFromResponseBody parses an OpenAI-shaped chat completion
// response body and estimates its cost from the rate table above. Returns
// null for a body with no parseable usage block (errors, blocked-by-
// guardrail, MCP responses) rather than treating it as zero-cost.
export function estimateCostFromResponseBody(body: string): UsageCost {
  let parsed: {
    model?: string;
    usage?: { prompt_tokens?: number; completion_tokens?: number };
  };
  try {
    parsed = JSON.parse(body);
  } catch {
    return null;
  }

  const model = parsed.model;
  const usage = parsed.usage;
  if (
    !model ||
    !usage ||
    typeof usage.prompt_tokens !== "number" ||
    typeof usage.completion_tokens !== "number"
  ) {
    return null;
  }

  const rate = RATES_PER_MILLION_TOKENS[model];
  if (!rate) return null;

  const costUsd =
    (usage.prompt_tokens / 1_000_000) * rate.input +
    (usage.completion_tokens / 1_000_000) * rate.output;

  return { costUsd, model };
}
