export function CostTicker({
  totalCostUsd,
  requestCount,
}: {
  totalCostUsd: number;
  requestCount: number;
}) {
  if (requestCount === 0) return null;

  return (
    <span
      title="Estimated from each response's token usage against the cost-attribution catalog's rates - not a live read of agentgateway's own accounting."
      className="inline-flex items-center gap-1.5 rounded-full border border-border bg-secondary px-3 py-1 text-xs"
    >
      <span className="font-medium tabular-nums text-foreground">
        ${totalCostUsd.toFixed(4)}
      </span>
      <span className="text-subtle">·</span>
      <span className="tabular-nums text-muted-foreground">
        {requestCount} request{requestCount === 1 ? "" : "s"}
      </span>
    </span>
  );
}
