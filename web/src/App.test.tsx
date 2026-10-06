import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "./App";

function mockFetch(scenarios: unknown, config: unknown) {
  return ((url: RequestInfo | URL) => {
    if (url === "/api/config") {
      return Promise.resolve({
        ok: true,
        json: async () => config,
      } as unknown as Response);
    }
    return Promise.resolve({
      ok: true,
      json: async () => scenarios,
    } as unknown as Response);
  }) as unknown as typeof fetch;
}

const emptyConfig = { gatewayHost: "", keycloakHost: "", missing: [] };

describe("App", () => {
  it("renders the wizard title", async () => {
    globalThis.fetch = mockFetch([], emptyConfig);
    render(<App />);
    expect(
      await screen.findByText("Agentgateway Demo Console"),
    ).toBeInTheDocument();
  });

  it("shows a setup banner listing missing environment variables", async () => {
    globalThis.fetch = mockFetch([], {
      ...emptyConfig,
      missing: ["AGW_HOST", "OPENAI_API_KEY"],
    });

    render(<App />);

    await waitFor(() =>
      expect(
        screen.getByText(/Missing environment variables:/),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText(/AGW_HOST, OPENAI_API_KEY/)).toBeInTheDocument();
  });

  it("hides Configure and Request & Response for a narrative step with nothing to demo", async () => {
    const pillars = [
      {
        id: "intro",
        title: "Welcome",
        steps: [
          {
            id: "welcome",
            title: "Welcome to Acme Corp",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
          },
        ],
      },
    ];
    globalThis.fetch = mockFetch(pillars, emptyConfig);

    render(<App />);

    expect(await screen.findByText("Welcome to Acme Corp")).toBeInTheDocument();
    expect(screen.queryByText("Configure")).not.toBeInTheDocument();
    expect(screen.queryByText("Request & Response")).not.toBeInTheDocument();
  });

  it("shows an error message when scenarios fail to load", async () => {
    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: async () => emptyConfig,
        } as unknown as Response);
      }
      return Promise.reject(new Error("network down"));
    }) as unknown as typeof fetch;

    render(<App />);

    await waitFor(() =>
      expect(
        screen.getByText("Failed to load the demo script: network down"),
      ).toBeInTheDocument(),
    );
  });
});
