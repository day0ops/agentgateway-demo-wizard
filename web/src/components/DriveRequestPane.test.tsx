import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { RequestPreset } from "../lib/api";
import { DriveRequestPane } from "./DriveRequestPane";

const preset: RequestPreset = {
  id: "team-alpha-ask",
  title: "team-alpha asks a question",
  identity: "team-alpha",
  method: "POST",
  path: "/openai",
  headers: {},
  body: '{"model":"gpt-4o-mini"}',
  stream: false,
  requiresSessionToken: false,
  exchangeViaSts: false,
};

describe("DriveRequestPane", () => {
  it("renders nothing when there are no presets", () => {
    const { container } = render(<DriveRequestPane presets={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("loads a preset and sends it, showing the raw response", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({
          statusCode: 200,
          body: '{"id":"chatcmpl-1"}',
          latencyMs: 42,
        }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<DriveRequestPane presets={[preset]} />);
    fireEvent.click(screen.getByText("team-alpha asks a question"));
    fireEvent.click(screen.getByText("Send"));

    await waitFor(() =>
      expect(screen.getByText("200 · 42ms")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        (_, element) =>
          element?.tagName === "CODE" &&
          element.textContent === '{\n  "id": "chatcmpl-1"\n}',
      ),
    ).toBeInTheDocument();
  });

  it("forwards the preset's headers in the request body sent to /api/request", async () => {
    const headerPreset: RequestPreset = {
      ...preset,
      id: "routing-by-name-gpt",
      headers: { "X-Demo-Model": "gpt-4o-mini" },
    };

    const calls: Parameters<typeof fetch>[] = [];
    globalThis.fetch = (async (...args: Parameters<typeof fetch>) => {
      calls.push(args);
      return {
        ok: true,
        json: async () => ({
          statusCode: 200,
          body: '{"id":"chatcmpl-1"}',
          latencyMs: 42,
        }),
      } as unknown as Response;
    }) as unknown as typeof fetch;

    render(<DriveRequestPane presets={[headerPreset]} />);
    fireEvent.click(screen.getByText("team-alpha asks a question"));
    fireEvent.click(screen.getByText("Send"));

    await waitFor(() => expect(calls.length).toBe(1));
    const sentBody = JSON.parse(calls[0][1]?.body as string);
    expect(sentBody.headers).toEqual({ "X-Demo-Model": "gpt-4o-mini" });
  });

  it("shows an error message when the request fails", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: false,
        status: 502,
        json: async () => ({
          error: "calling gateway: dial tcp: connection refused",
        }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<DriveRequestPane presets={[preset]} />);
    fireEvent.click(screen.getByText("team-alpha asks a question"));
    fireEvent.click(screen.getByText("Send"));

    await waitFor(() =>
      expect(
        screen.getByText("calling gateway: dial tcp: connection refused"),
      ).toBeInTheDocument(),
    );
  });

  it("streams an incremental response when the preset requests streaming", async () => {
    const streamPreset: RequestPreset = {
      ...preset,
      id: "ask-streaming",
      stream: true,
    };

    const chunks = ["data: chunk1\n\n", "data: chunk2\n\n"];
    let i = 0;
    const reader = {
      read: async () => {
        if (i >= chunks.length) return { done: true, value: undefined };
        const value = new TextEncoder().encode(chunks[i]);
        i += 1;
        return { done: false, value };
      },
    };
    globalThis.fetch = (async () =>
      ({
        ok: true,
        status: 200,
        body: { getReader: () => reader },
      }) as unknown as Response) as unknown as typeof fetch;

    render(<DriveRequestPane presets={[streamPreset]} />);
    fireEvent.click(screen.getByText(streamPreset.title));
    fireEvent.click(screen.getByText("Send"));

    await waitFor(() => expect(screen.getByText(/chunk1/)).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText(/chunk2/)).toBeInTheDocument());
  });

  it("calls onResponse with the raw response body after a successful send", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({
          statusCode: 200,
          body: '{"model":"gpt-4o-mini","usage":{"prompt_tokens":10,"completion_tokens":5}}',
          latencyMs: 42,
        }),
      }) as unknown as Response) as unknown as typeof fetch;

    const onResponse = vi.fn();
    render(<DriveRequestPane presets={[preset]} onResponse={onResponse} />);
    fireEvent.click(screen.getByText("team-alpha asks a question"));
    fireEvent.click(screen.getByText("Send"));

    await waitFor(() =>
      expect(onResponse).toHaveBeenCalledWith(
        '{"model":"gpt-4o-mini","usage":{"prompt_tokens":10,"completion_tokens":5}}',
      ),
    );
  });
});
