import { beforeEach, describe, expect, it } from "vitest";
import { getTheme, initTheme, setTheme } from "./theme";

describe("theme", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
  });

  it("defaults to light when nothing is stored and there is no dark preference", () => {
    expect(getTheme()).toBe("light");
  });

  it("persists and applies a chosen theme", () => {
    setTheme("dark");
    expect(localStorage.getItem("agw-demo-wizard-theme")).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("initTheme applies the stored theme to the document and returns it", () => {
    localStorage.setItem("agw-demo-wizard-theme", "dark");
    expect(initTheme()).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });
});
