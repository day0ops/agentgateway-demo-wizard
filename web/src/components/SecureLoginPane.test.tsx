import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SecureLoginPane } from "./SecureLoginPane";

describe("SecureLoginPane", () => {
  it("renders a login link pointing at the ext-auth-gated trigger path", () => {
    render(<SecureLoginPane />);
    expect(screen.getByRole("link", { name: /log in/i })).toHaveAttribute(
      "href",
      "/secure/login?returnTo=auth",
    );
  });

  it("renders a logout link", () => {
    render(<SecureLoginPane />);
    expect(screen.getByRole("link", { name: /log out/i })).toHaveAttribute(
      "href",
      "/secure/logout",
    );
  });
});
