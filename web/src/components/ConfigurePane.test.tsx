import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ConfigurePane } from "./ConfigurePane";

function mockStreamingFetch(events: string[]) {
  let i = 0;
  const reader = {
    read: async () => {
      if (i >= events.length) return { done: true, value: undefined };
      const value = new TextEncoder().encode(events[i]);
      i += 1;
      return { done: false, value };
    },
  };
  return () =>
    Promise.resolve({
      ok: true,
      body: { getReader: () => reader },
    } as unknown as Response);
}

describe("ConfigurePane", () => {
  it("renders nothing when there are no policies", () => {
    const { container } = render(<ConfigurePane policies={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("applies a policy and shows the applied state with a revert button", async () => {
    globalThis.fetch = mockStreamingFetch([
      'data: {"type":"progress","message":"Applying..."}\n\n',
      'data: {"type":"done","message":"Applied 2 resource(s)","yaml":"kind: Foo"}\n\n',
    ]) as unknown as typeof fetch;

    render(
      <ConfigurePane
        policies={[{ id: "fallback", title: "Automatic fallback" }]}
      />,
    );
    fireEvent.click(screen.getByText("Apply"));

    await waitFor(() =>
      expect(screen.getByText("● applied")).toBeInTheDocument(),
    );
    expect(screen.getByText("Revert")).toBeInTheDocument();
  });

  it("shows the applied YAML expanded by default, without needing to click", async () => {
    globalThis.fetch = mockStreamingFetch([
      'data: {"type":"done","message":"Applied 1 resource(s)","yaml":"kind: Foo"}\n\n',
    ]) as unknown as typeof fetch;

    render(
      <ConfigurePane
        policies={[{ id: "fallback", title: "Automatic fallback" }]}
      />,
    );
    fireEvent.click(screen.getByText("Apply"));

    await waitFor(() =>
      expect(screen.getByText("kind: Foo")).toBeInTheDocument(),
    );
    expect(screen.getByText("hide YAML")).toBeInTheDocument();
  });

  it("shows a red error state when apply fails", async () => {
    globalThis.fetch = mockStreamingFetch([
      'data: {"type":"error","message":"boom"}\n\n',
    ]) as unknown as typeof fetch;

    render(
      <ConfigurePane policies={[{ id: "broken", title: "Broken policy" }]} />,
    );
    fireEvent.click(screen.getByText("Apply"));

    await waitFor(() => expect(screen.getByText("boom")).toBeInTheDocument());
  });

  it("surfaces a non-OK apply response instead of hanging on 'applying'", async () => {
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: false,
        status: 404,
        text: async () => 'unknown policy "x"',
      })) as unknown as typeof fetch;

    render(
      <ConfigurePane policies={[{ id: "missing", title: "Missing policy" }]} />,
    );
    fireEvent.click(screen.getByText("Apply"));

    await waitFor(() =>
      expect(screen.getByText('unknown policy "x"')).toBeInTheDocument(),
    );
    expect(screen.getByText("● error")).toBeInTheDocument();
  });

  it("surfaces a stream read failure instead of hanging on 'applying'", async () => {
    const reader = {
      read: async () => {
        throw new Error("stream disconnected");
      },
    };
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: true,
        body: { getReader: () => reader },
      })) as unknown as typeof fetch;

    render(
      <ConfigurePane
        policies={[{ id: "fallback", title: "Automatic fallback" }]}
      />,
    );
    fireEvent.click(screen.getByText("Apply"));

    await waitFor(() =>
      expect(screen.getByText("stream disconnected")).toBeInTheDocument(),
    );
    expect(screen.getByText("● error")).toBeInTheDocument();
    expect(screen.getByText("Apply")).not.toBeDisabled();
  });

  it("surfaces a non-OK revert response and keeps the Revert button", async () => {
    let i = 0;
    const events = [
      'data: {"type":"done","message":"Applied 1 resource(s)","yaml":"kind: Foo"}\n\n',
    ];
    const reader = {
      read: async () => {
        if (i >= events.length) return { done: true, value: undefined };
        const value = new TextEncoder().encode(events[i]);
        i += 1;
        return { done: false, value };
      },
    };
    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config/apply") {
        return Promise.resolve({
          ok: true,
          body: { getReader: () => reader },
        });
      }
      return Promise.resolve({
        ok: false,
        status: 500,
        json: async () => ({ error: "revert failed: stuck delete" }),
      });
    }) as unknown as typeof fetch;

    render(
      <ConfigurePane
        policies={[{ id: "fallback", title: "Automatic fallback" }]}
      />,
    );
    fireEvent.click(screen.getByText("Apply"));
    await waitFor(() => expect(screen.getByText("Revert")).toBeInTheDocument());

    fireEvent.click(screen.getByText("Revert"));

    await waitFor(() =>
      expect(
        screen.getByText("revert failed: stuck delete"),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("Revert")).toBeInTheDocument();
  });
});
