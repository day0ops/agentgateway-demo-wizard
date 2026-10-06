import { test, expect } from "@playwright/test";

const scenarios = [
  {
    id: "intro",
    title: "Welcome",
    steps: [
      {
        id: "welcome",
        title: "Welcome to Acme Corp",
        explanation:
          "Acme Corp gives every team safe access to LLMs through one gateway.",
        diagram: "",
        policies: [],
        presets: [],
        agentDemo: false,
      },
    ],
  },
  {
    id: "routing",
    title: "One endpoint, many models",
    steps: [
      {
        id: "routing-by-name",
        title: "Model routing by name",
        explanation: "One endpoint resolves to different models by name.",
        diagram: "",
        policies: [{ id: "routing-by-name", title: "Model-by-name routing" }],
        presets: [
          {
            id: "ask-gpt-4o-mini",
            title: "Ask for gpt-4o-mini",
            identity: "anonymous",
            method: "POST",
            path: "/chat/by-name",
            headers: {},
            body: "{}",
          },
        ],
        agentDemo: false,
      },
    ],
  },
  {
    id: "agent",
    title: "From models to agents",
    steps: [
      {
        id: "agent-mcp-server",
        title: "Agent to MCP tool call",
        explanation: "Run the agent and watch it call a tool.",
        diagram: "",
        policies: [],
        presets: [],
        agentDemo: true,
      },
    ],
  },
  {
    id: "wrap",
    title: "Wrap-up",
    steps: [
      {
        id: "recap",
        title: "One gateway, four guarantees",
        explanation: "Recap text.",
        diagram: "",
        policies: [],
        presets: [],
        agentDemo: false,
      },
    ],
  },
];

test.beforeEach(async ({ page }) => {
  await page.route("**/api/config", (route) =>
    route.fulfill({
      json: {
        gatewayHost: "agw.example.com",
        keycloakHost: "keycloak.example.com",
        missing: [],
      },
    }),
  );
  await page.route("**/api/scenarios", (route) =>
    route.fulfill({ json: scenarios }),
  );
});

test("breadcrumb navigation moves between pillars", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Welcome to Acme Corp")).toBeVisible();

  await page.getByRole("button", { name: "One endpoint, many models" }).click();
  await expect(page.getByText("Model routing by name")).toBeVisible();
  await expect(page.getByText("step 1 of 1")).toBeVisible();
});

test("applying a policy shows the applied state and a revert button", async ({
  page,
}) => {
  await page.route("**/api/config/apply", (route) =>
    route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: 'data: {"type":"done","message":"Applied 1 resource(s)","yaml":"kind: Foo"}\n\n',
    }),
  );

  await page.goto("/");
  await page.getByRole("button", { name: "One endpoint, many models" }).click();
  await page.getByRole("button", { name: "Apply" }).click();

  await expect(page.getByText("● applied")).toBeVisible();
  await expect(page.getByRole("button", { name: "Revert" })).toBeVisible();
});

test("sending a request shows the raw response", async ({ page }) => {
  await page.route("**/api/request", (route) =>
    route.fulfill({
      json: { statusCode: 200, body: '{"id":"chatcmpl-1"}', latencyMs: 42 },
    }),
  );

  await page.goto("/");
  await page.getByRole("button", { name: "One endpoint, many models" }).click();
  await page.getByRole("button", { name: "Ask for gpt-4o-mini" }).click();
  await page.getByRole("button", { name: "Send" }).click();

  await expect(page.getByText("200 · 42ms")).toBeVisible();
  await expect(page.getByText('{"id":"chatcmpl-1"}')).toBeVisible();
});

test("the agent loop streams hops to a final answer", async ({ page }) => {
  await page.route("**/api/agent/run", (route) =>
    route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body:
        'data: {"type":"model_call","message":"Calling the model (hop 1)..."}\n\n' +
        'data: {"type":"final","message":"AAPL is trading at $150."}\n\n',
    }),
  );

  await page.goto("/");
  await page.getByRole("button", { name: "From models to agents" }).click();
  await page.getByRole("button", { name: "Run agent" }).click();

  await expect(page.getByText("AAPL is trading at $150.")).toBeVisible();
});
