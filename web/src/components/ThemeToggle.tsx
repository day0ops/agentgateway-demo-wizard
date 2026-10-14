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
      className="rounded px-2 py-1 text-sm transition-transform duration-150 ease-out hover:bg-slate-100 active:scale-[0.9] dark:hover:bg-slate-800"
    >
      {theme === "dark" ? "☾" : "☀"}
    </button>
  );
}
