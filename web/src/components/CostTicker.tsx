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
      className="rounded-full bg-indigo-100 px-3 py-1 text-xs font-medium text-indigo-700 dark:bg-indigo-950 dark:text-indigo-300"
    >
      ${totalCostUsd.toFixed(4)} · {requestCount} request
      {requestCount === 1 ? "" : "s"}
    </span>
  );
}
