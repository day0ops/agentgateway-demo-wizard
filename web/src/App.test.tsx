import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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

const emptyConfig = {
  gatewayHost: "",
  keycloakHost: "",
  missing: [],
  version: "",
};

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

  it("renders the version from /api/config in the footer", async () => {
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
    globalThis.fetch = mockFetch(pillars, {
      ...emptyConfig,
      version: "v0.2.0",
    });
    render(<App />);
    expect(await screen.findByText("v0.2.0")).toBeInTheDocument();
  });

  it("clears the drive-request pane's response when navigating to the next step", async () => {
    const stepTemplate = {
      explanation: "",
      diagram: "",
      policies: [],
      agentDemo: false,
    };
    const pillars = [
      {
        id: "routing",
        title: "Routing",
        steps: [
          {
            ...stepTemplate,
            id: "step-one",
            title: "Step one",
            presets: [
              {
                id: "ask",
                title: "Ask",
                identity: "anonymous",
                method: "POST",
                path: "/chat",
                headers: {},
                body: "{}",
                stream: false,
              },
            ],
          },
          {
            ...stepTemplate,
            id: "step-two",
            title: "Step two",
            presets: [
              {
                id: "ask-two",
                title: "Ask again",
                identity: "anonymous",
                method: "POST",
                path: "/chat",
                headers: {},
                body: "{}",
                stream: false,
              },
            ],
          },
        ],
      },
    ];

    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: async () => emptyConfig,
        } as unknown as Response);
      }
      if (url === "/api/request") {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            statusCode: 200,
            body: '{"id":"chatcmpl-1"}',
            latencyMs: 10,
          }),
        } as unknown as Response);
      }
      return Promise.resolve({
        ok: true,
        json: async () => pillars,
      } as unknown as Response);
    }) as unknown as typeof fetch;

    render(<App />);

    expect(await screen.findByText("Step one")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Ask"));
    fireEvent.click(screen.getByText("Send"));
    await waitFor(() =>
      expect(screen.getByText("200 · 10ms")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByText("next"));
    expect(await screen.findByText("Step two")).toBeInTheDocument();
    expect(screen.queryByText("200 · 10ms")).not.toBeInTheDocument();
    expect(screen.queryByText("Ask")).not.toBeInTheDocument();
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

  it("shows the login pane for a step with loginDemo instead of Configure/Request panes", async () => {
    const pillars = [
      {
        id: "auth",
        title: "Who's allowed to do what",
        teaser: "",
        steps: [
          {
            id: "auth-login",
            title: "Real login",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
            virtualKeys: false,
            loginDemo: true,
          },
        ],
      },
    ];
    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: async () => emptyConfig,
        } as unknown as Response);
      }
      if (typeof url === "string" && url.startsWith("/api/auth/status")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ loggedIn: false }),
        } as unknown as Response);
      }
      return Promise.resolve({
        ok: true,
        json: async () => pillars,
      } as unknown as Response);
    }) as unknown as typeof fetch;

    render(<App />);

    expect(await screen.findByText("Real login")).toBeInTheDocument();
    expect(
      await screen.findByRole("link", { name: /log in as team-alpha/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Configure")).not.toBeInTheDocument();
  });

  it("restores the matching pillar when the URL carries a returnTo param", async () => {
    const pillars = [
      {
        id: "intro",
        title: "Welcome",
        teaser: "",
        steps: [
          {
            id: "welcome",
            title: "Welcome",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
            virtualKeys: false,
            loginDemo: false,
          },
        ],
      },
      {
        id: "auth",
        title: "Who's allowed to do what",
        teaser: "",
        steps: [
          {
            id: "auth-login",
            title: "Real login",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
            virtualKeys: false,
            loginDemo: true,
          },
        ],
      },
    ];
    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: async () => emptyConfig,
        } as unknown as Response);
      }
      if (typeof url === "string" && url.startsWith("/api/auth/status")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ loggedIn: false }),
        } as unknown as Response);
      }
      return Promise.resolve({
        ok: true,
        json: async () => pillars,
      } as unknown as Response);
    }) as unknown as typeof fetch;

    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      value: { ...originalLocation, search: "?returnTo=auth" },
      writable: true,
    });

    render(<App />);

    expect(await screen.findByText("Real login")).toBeInTheDocument();

    Object.defineProperty(window, "location", {
      value: originalLocation,
      writable: true,
    });
  });
});
