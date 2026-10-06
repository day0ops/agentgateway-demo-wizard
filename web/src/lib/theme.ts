// The storage key here must match the inline pre-paint script in index.html
// byte-for-byte - that script runs before this module loads, so the key
// cannot be shared as a real constant across the two.
const STORAGE_KEY = "agw-demo-wizard-theme";

// Fired on every setTheme call so components outside the ThemeToggle (e.g.
// a diagram that needs to re-render in the new color scheme) can react to a
// theme change they didn't trigger themselves.
const THEME_CHANGE_EVENT = "agw-demo-wizard-theme-change";

export type Theme = "light" | "dark";

export function getTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

export function setTheme(theme: Theme) {
  localStorage.setItem(STORAGE_KEY, theme);
  applyTheme(theme);
  window.dispatchEvent(new Event(THEME_CHANGE_EVENT));
}

export function initTheme(): Theme {
  const theme = getTheme();
  applyTheme(theme);
  return theme;
}

// Subscribes to theme changes made via setTheme elsewhere in the app. Returns
// an unsubscribe function.
export function onThemeChange(listener: () => void): () => void {
  window.addEventListener(THEME_CHANGE_EVENT, listener);
  return () => window.removeEventListener(THEME_CHANGE_EVENT, listener);
}
