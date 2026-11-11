import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import type { Plugin, Connect } from "vite";

// Dev-only mock of the Go backend, so `npm run dev` renders the full wizard
// without a live cluster, Keycloak, or agentgateway. It serves the real
// scenario registry (dumped to scenarios.json) plus canned responses for the
// interactive panes, enough to exercise every rendered state for design work.
// Never active in a production build: the plugin applies only in serve mode.

const here = dirname(fileURLToPath(import.meta.url));
const readFixture = (name: string) =>
  readFileSync(join(here, name), "utf-8");

function json(res: Parameters<Connect.NextHandleFunction>[1], body: unknown) {
  res.setHeader("Content-Type", "application/json");
  res.end(typeof body === "string" ? body : JSON.stringify(body));
}

function sse(res: Parameters<Connect.NextHandleFunction>[1], events: unknown[]) {
  res.setHeader("Content-Type", "text/event-stream");
  res.end(events.map((e) => `data: ${JSON.stringify(e)}\n\n`).join(""));
}

const SAMPLE_YAML = `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: chat-by-name
  namespace: agentgateway
spec:
  parentRefs:
    - name: agentgateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /chat/by-name
      backendRefs:
        - name: openai
          port: 443`;

const SAMPLE_CHAT = JSON.stringify(
  {
    id: "chatcmpl-9e2f1a7c",
    object: "chat.completion",
    model: "gpt-4o-mini",
    choices: [
      {
        index: 0,
        message: {
          role: "assistant",
          content: "Routed to gpt-4o-mini. Everything is working as configured.",
        },
        finish_reason: "stop",
      },
    ],
    usage: { prompt_tokens: 18, completion_tokens: 24, total_tokens: 42 },
  },
  null,
  2,
);

export function mockApi(): Plugin {
  return {
    name: "demo-wizard-mock-api",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = (req.url ?? "").split("?")[0];
        const method = req.method ?? "GET";

        if (url === "/api/scenarios") return json(res, readFixture("scenarios.json"));
        if (url === "/api/config") return json(res, readFixture("config.json"));

        if (url === "/api/config/view") {
          return json(res, {
            refs: [{ name: "httproute/chat-by-name", found: true, yaml: SAMPLE_YAML }],
          });
        }
        if (url === "/api/config/apply" && method === "POST") {
          return sse(res, [
            { type: "done", message: "Applied 1 resource(s)", yaml: SAMPLE_YAML },
          ]);
        }
        if (url === "/api/config/revert" && method === "POST") return json(res, {});

        if (url === "/api/request" && method === "POST") {
          return json(res, { statusCode: 200, body: SAMPLE_CHAT, latencyMs: 418 });
        }

        if (url === "/api/agent/run" && method === "POST") {
          return sse(res, [
            { type: "model_call", message: "Calling the model (hop 1)..." },
            {
              type: "tool_call",
              message: "get_stock_price(symbol=AAPL)",
              detail: '{"symbol":"AAPL"}',
            },
            { type: "tool_result", message: "get_stock_price returned $228.52" },
            { type: "model_call", message: "Calling the model (hop 2)..." },
            {
              type: "final",
              message: "AAPL is trading at $228.52 right now.",
            },
          ]);
        }

        if (url === "/api/virtual-keys" && method === "POST") {
          return json(res, {
            name: "team-gamma",
            key: "sk-agw-7f3c9a21d8e04b56",
            userId: "payer-gamma",
          });
        }
        if (url.startsWith("/api/virtual-keys/") && url.endsWith("/rotate")) {
          return json(res, {
            name: "team-gamma",
            key: "sk-agw-1b84e0d9c7a2f365",
            userId: "payer-gamma",
          });
        }

        if (url === "/secure/session") return json(res, { identity: "diana.prince@acme.example" });
        if (url === "/secure/ops-agent/run" && method === "POST") {
          return json(res, {
            answer:
              "Your balance across both checking accounts is **$48,210.74**. The most recent transfer of $1,200.00 to settlement cleared 2 hours ago.",
            extra: {
              identity: "diana.prince@acme.example",
              chain: {
                sub: "diana.prince@acme.example",
                act: "transfer-orchestrator",
                act_act: "settlement-agent",
              },
            },
          });
        }
        if (url === "/secure/direct-call" && method === "POST") {
          return json(res, {
            statusCode: 403,
            body: JSON.stringify({
              error: { code: -32601, message: "tool not found: settle_transfer" },
            }),
          });
        }

        if (url === "/api/toolmode/run" && method === "POST") {
          type ToolModeFixture = {
            p: number;
            c: number;
            hops: number;
            toolsExposed: string[];
            toolInvocations: { name: string; arguments: string }[];
          };
          const RUN_CODE_SNIPPET = JSON.stringify({
            code: [
              "const holdings = await tools.get_holdings();",
              'const aapl = await tools.get_stock_price({ symbol: "AAPL" });',
              'const msft = await tools.get_stock_price({ symbol: "MSFT" });',
              'const rate = await tools.get_exchange_rate({ from: "USD", to: "EUR" });',
              "return (holdings.AAPL * aapl + holdings.MSFT * msft) * rate;",
            ].join("\n"),
          });
          const byMode: Record<string, ToolModeFixture> = {
            standard: {
              p: 4200,
              c: 180,
              hops: 4,
              toolsExposed: [
                "get_holdings",
                "get_stock_price",
                "get_exchange_rate",
                "get_account_balance",
              ],
              toolInvocations: [
                { name: "get_holdings", arguments: "{}" },
                { name: "get_stock_price", arguments: '{"symbol":"AAPL"}' },
                { name: "get_stock_price", arguments: '{"symbol":"MSFT"}' },
                {
                  name: "get_exchange_rate",
                  arguments: '{"from":"USD","to":"EUR"}',
                },
              ],
            },
            search: {
              p: 1400,
              c: 150,
              hops: 5,
              toolsExposed: ["get_tool", "invoke_tool"],
              toolInvocations: [
                { name: "get_tool", arguments: '{"name":"get_holdings"}' },
                { name: "invoke_tool", arguments: '{"name":"get_holdings","arguments":{}}' },
                { name: "get_tool", arguments: '{"name":"get_stock_price"}' },
              ],
            },
            code: {
              p: 1100,
              c: 120,
              hops: 2,
              toolsExposed: ["run_code"],
              toolInvocations: [
                { name: "run_code", arguments: RUN_CODE_SNIPPET },
              ],
            },
            codesearch: {
              p: 950,
              c: 110,
              hops: 2,
              toolsExposed: ["get_tool", "run_code"],
              toolInvocations: [
                { name: "run_code", arguments: RUN_CODE_SNIPPET },
              ],
            },
          };
          let body = "";
          req.on("data", (chunk) => (body += chunk));
          req.on("end", () => {
            const mode = (() => {
              try {
                return JSON.parse(body).mode as string;
              } catch {
                return "standard";
              }
            })();
            const m = byMode[mode] ?? byMode.standard;
            json(res, {
              answer:
                "Your AAPL + MSFT holdings are worth about EUR 18,640 right now.",
              promptTokens: m.p,
              completionTokens: m.c,
              hops: m.hops,
              toolCalls: m.toolInvocations.length,
              toolsExposed: m.toolsExposed,
              toolInvocations: m.toolInvocations,
            });
          });
          return;
        }

        next();
      });
    },
  };
}
