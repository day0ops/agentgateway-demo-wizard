import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { setTheme } from "../lib/theme";
import { CodeBlock } from "./CodeBlock";

describe("CodeBlock", () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
  });

  it("uses a themed background container", () => {
    const { container } = render(<CodeBlock language="json" code='{"a": 1}' />);
    const pre = container.querySelector("pre");
    expect(pre?.className).toContain("bg-elevated");
  });

  it("re-renders token colors in the new theme when it toggles", async () => {
    render(<CodeBlock language="json" code='{"a": 1}' />);
    const lightColor = screen.getByText('"a"').getAttribute("style");

    act(() => {
      setTheme("dark");
    });

    await waitFor(() => {
      const darkColor = screen.getByText('"a"').getAttribute("style");
      expect(darkColor).not.toBe(lightColor);
    });
  });
});
