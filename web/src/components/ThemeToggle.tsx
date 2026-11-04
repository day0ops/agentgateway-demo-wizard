import { useState } from "react";
import { Moon, Sun } from "lucide-react";
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
      aria-label={
        theme === "dark" ? "Switch to light theme" : "Switch to dark theme"
      }
      className="flex size-8 items-center justify-center rounded-lg border border-border bg-card text-muted-foreground transition-colors duration-150 ease-out hover:bg-secondary hover:text-foreground active:scale-[0.92]"
    >
      {theme === "dark" ? <Moon size={16} /> : <Sun size={16} />}
    </button>
  );
}
