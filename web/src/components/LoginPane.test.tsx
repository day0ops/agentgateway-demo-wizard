import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LoginPane } from "./LoginPane";

describe("LoginPane", () => {
  it("shows a login link for an identity with no captured token", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({ loggedIn: false }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<LoginPane identities={["team-alpha"]} />);

    expect(
      await screen.findByRole("link", { name: /log in as team-alpha/i }),
    ).toHaveAttribute(
      "href",
      "/auth/login?identity=team-alpha&return_to=auth",
    );
  });

  it("shows a logged-in badge once a real token has been captured", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({ loggedIn: true }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<LoginPane identities={["team-alpha"]} />);

    expect(
      await screen.findByText(/logged in \(real token\)/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /log in as team-alpha/i }),
    ).not.toBeInTheDocument();
  });

  it("checks status independently for each identity", async () => {
    globalThis.fetch = ((url: RequestInfo | URL) => {
      const loggedIn = String(url).includes("team-alpha");
      return Promise.resolve({
        ok: true,
        json: async () => ({ loggedIn }),
      } as unknown as Response);
    }) as unknown as typeof fetch;

    render(<LoginPane identities={["team-alpha", "team-beta"]} />);

    expect(
      await screen.findByText(/logged in \(real token\)/i),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("link", { name: /log in as team-beta/i }),
    ).toBeInTheDocument();
  });
});
