export type Policy = {
  id: string;
  title: string;
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

export type Step = {
  id: string;
  title: string;
  explanation: string;
  diagram: string;
  policies: Policy[];
  presets: RequestPreset[];
  agentDemo: boolean;
  virtualKeys: boolean;
};

export type Pillar = {
  id: string;
  title: string;
  steps: Step[];
};

export type AppConfig = {
  gatewayHost: string;
  keycloakHost: string;
  missing: string[];
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
