import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SecureLoginPane } from "./SecureLoginPane";

function mockSessionFetch({
  loggedIn,
  identity,
}: {
  loggedIn: boolean;
  identity?: string;
}) {
  return (() =>
    Promise.resolve(
      loggedIn
        ? ({
            type: "basic",
            ok: true,
            json: async () => ({ identity }),
          } as unknown as Response)
        : ({ type: "opaqueredirect", ok: false } as unknown as Response),
    )) as unknown as typeof fetch;
}

describe("SecureLoginPane", () => {
  it("renders a login link pointing at the returnTo step while logged out", async () => {
    globalThis.fetch = mockSessionFetch({ loggedIn: false });
    render(<SecureLoginPane returnToStepId="auth-login" />);

    await waitFor(() =>
      expect(screen.getByRole("link", { name: /log in/i })).toHaveAttribute(
        "href",
        "/secure/login?returnTo=auth-login",
      ),
    );
  });

  it("shows the logged-in identity once the session check succeeds", async () => {
    globalThis.fetch = mockSessionFetch({
      loggedIn: true,
      identity: "team-alpha",
    });
    render(<SecureLoginPane returnToStepId="auth-login" />);

    await waitFor(() =>
      expect(screen.getByText(/logged in as team-alpha/i)).toBeInTheDocument(),
    );
    expect(
      screen.queryByRole("link", { name: /^log in$/i }),
    ).not.toBeInTheDocument();
  });

  it("renders a logout link regardless of session state", async () => {
    globalThis.fetch = mockSessionFetch({ loggedIn: false });
    render(<SecureLoginPane returnToStepId="auth-login" />);

    await waitFor(() =>
      expect(screen.getByRole("link", { name: /log out/i })).toHaveAttribute(
        "href",
        "/secure/logout",
      ),
    );
  });
});
