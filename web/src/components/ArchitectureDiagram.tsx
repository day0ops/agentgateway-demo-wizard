// Static block diagram of the banking agents + MCP backends, authored in
// draw.io (source: docs/diagrams/agents-mcp-architecture.drawio) and
// exported to SVG - shown once, on the welcome step, as an overview before
// the pillar walkthroughs below it.
export function ArchitectureDiagram() {
  return (
    <div className="px-6 pb-6">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
        Agents + MCP architecture
      </h2>
      <div className="rounded-xl border border-slate-200 p-4 shadow-sm dark:border-zinc-800">
        <img
          src="/diagrams/agents-mcp-architecture.svg"
          alt="Block diagram: every call passes through agentgateway, which enforces routing, auth, guardrails, and cost. You call through agentgateway to reach the Accounts, Transfer Orchestrator, Settlement, and Statements agents; the Transfer Orchestrator's delegation to the Settlement agent is also routed through agentgateway, not a direct call; and all four agents reach the Core Banking MCP and Stripe MCP (sandbox) backends through agentgateway as well."
          className="mx-auto w-full max-w-[1300px]"
        />
      </div>
    </div>
  );
}
