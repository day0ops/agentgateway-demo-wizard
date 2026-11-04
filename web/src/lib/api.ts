export type Policy = {
  id: string;
  title: string;
  readOnly: boolean;
};

export type RequestPreset = {
  id: string;
  title: string;
  identity: string;
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  stream: boolean;
};

export type OpsAgentDemo = {
  agent: string;
  mode: string;
  defaultPrompt: string;
};

export type DirectCallDemo = {
  title: string;
  path: string;
  tool: string;
  arguments: Record<string, unknown>;
};

export type ToolMode = {
  key: string;
  label: string;
  mcpPath: string;
  description: string;
};

export type ToolModeDemo = {
  defaultPrompt: string;
  modes: ToolMode[];
};

export type ToolModeResult = {
  answer: string;
  promptTokens: number;
  completionTokens: number;
  hops: number;
  toolCalls: number;
};

export type Step = {
  id: string;
  title: string;
  explanation: string;
  diagram: string;
  policies: Policy[];
  presets: RequestPreset[];
  agentDemo: boolean;
  virtualKeys: boolean;
  loginDemo: boolean;
  opsAgent?: OpsAgentDemo;
  directCall?: DirectCallDemo;
  toolModeDemo?: ToolModeDemo;
};

export type Pillar = {
  id: string;
  title: string;
  teaser: string;
  steps: Step[];
};

export type AppConfig = {
  gatewayHost: string;
  keycloakHost: string;
  missing: string[];
  version: string;
};

export async function fetchScenarios(): Promise<Pillar[]> {
  const res = await fetch("/api/scenarios");
  if (!res.ok) throw new Error(`failed to load scenarios: ${res.status}`);
  return res.json();
}

export async function fetchConfig(): Promise<AppConfig> {
  const res = await fetch("/api/config");
  if (!res.ok) throw new Error(`failed to load config: ${res.status}`);
  return res.json();
}

export async function runToolMode(
  mode: string,
  prompt: string,
): Promise<ToolModeResult> {
  const res = await fetch("/api/toolmode/run", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode, prompt }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
