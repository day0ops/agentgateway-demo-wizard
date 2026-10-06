import { useState } from "react";
import { getTheme, setTheme, type Theme } from "../lib/theme";

export function ThemeToggle() {
  const [theme, setThemeState] = useState<Theme>(getTheme);

  function toggle() {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    setThemeState(next);
  }

  return (
    <button
      onClick={toggle}
      aria-label="Toggle color theme"
      className="rounded px-2 py-1 text-sm"
    >
      {theme === "dark" ? "☾" : "☀"}
    </button>
  );
}
