import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { VirtualKeyPane } from "./VirtualKeyPane";

function fillForm() {
  fireEvent.change(screen.getByPlaceholderText("Key name (e.g. team-gamma)"), {
    target: { value: "team-gamma" },
  });
  fireEvent.change(screen.getByPlaceholderText("User id"), {
    target: { value: "team-gamma" },
  });
}

describe("VirtualKeyPane", () => {
  it("creates a key and shows it in the list", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({
          name: "team-gamma",
          key: "sk-team-gamma-abc123",
          userId: "team-gamma",
        }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<VirtualKeyPane />);
    fillForm();
    fireEvent.click(screen.getByText("Create key"));

    await waitFor(() =>
      expect(screen.getByText("sk-team-gamma-abc123")).toBeInTheDocument(),
    );
  });

  it("rotates a key and shows the new value", async () => {
    let call = 0;
    globalThis.fetch = (async () => {
      call += 1;
      if (call === 1) {
        return {
          ok: true,
          json: async () => ({
            name: "team-gamma",
            key: "sk-team-gamma-old",
            userId: "team-gamma",
          }),
        } as unknown as Response;
      }
      return {
        ok: true,
        json: async () => ({ name: "team-gamma", key: "sk-team-gamma-new" }),
      } as unknown as Response;
    }) as unknown as typeof fetch;

    render(<VirtualKeyPane />);
    fillForm();
    fireEvent.click(screen.getByText("Create key"));
    await waitFor(() =>
      expect(screen.getByText("sk-team-gamma-old")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByText("Rotate"));
    await waitFor(() =>
      expect(screen.getByText("sk-team-gamma-new")).toBeInTheDocument(),
    );
  });

  it("shows an error message when creation fails", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: false,
        text: async () => "apply the Budgets and spend limits policy first",
      }) as unknown as Response) as unknown as typeof fetch;

    render(<VirtualKeyPane />);
    fillForm();
    fireEvent.click(screen.getByText("Create key"));

    await waitFor(() =>
      expect(
        screen.getByText("apply the Budgets and spend limits policy first"),
      ).toBeInTheDocument(),
    );
  });

  it("shows a clean message instead of a raw JSON blob when the backend returns a JSON error", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: false,
        text: async () =>
          '{"error":"the \\"cost-budgets-virtual-keys\\" Secret doesn\'t exist yet - apply the Budgets and spend limits policy first"}',
      }) as unknown as Response) as unknown as typeof fetch;

    render(<VirtualKeyPane />);
    fillForm();
    fireEvent.click(screen.getByText("Create key"));

    await waitFor(() =>
      expect(
        screen.getByText(
          'the "cost-budgets-virtual-keys" Secret doesn\'t exist yet - apply the Budgets and spend limits policy first',
        ),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText(/\{"error":/)).not.toBeInTheDocument();
  });
});
