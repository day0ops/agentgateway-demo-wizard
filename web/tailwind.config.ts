import type { Config } from "tailwindcss";
import defaultTheme from "tailwindcss/defaultTheme";

// Maps a --token (space-separated RGB channels, defined in index.css) to a
// Tailwind color that still honors opacity modifiers like bg-card/70.
const token = (name: string) => `rgb(var(--${name}) / <alpha-value>)`;

export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: token("background"),
        foreground: token("foreground"),
        card: token("card"),
        elevated: token("elevated"),
        muted: {
          DEFAULT: token("muted"),
          foreground: token("muted-foreground"),
        },
        subtle: token("subtle-foreground"),
        border: token("border"),
        "border-strong": token("border-strong"),
        input: token("input"),
        ring: token("ring"),
        primary: {
          DEFAULT: token("primary"),
          hover: token("primary-hover"),
          foreground: token("primary-foreground"),
        },
        secondary: {
          DEFAULT: token("secondary"),
          hover: token("secondary-hover"),
          foreground: token("secondary-foreground"),
        },
        accent: {
          DEFAULT: token("accent"),
          foreground: token("accent-foreground"),
        },
        success: {
          DEFAULT: token("success"),
          soft: token("success-soft"),
          border: token("success-border"),
        },
        warning: {
          DEFAULT: token("warning"),
          soft: token("warning-soft"),
          border: token("warning-border"),
        },
        info: {
          DEFAULT: token("info"),
          soft: token("info-soft"),
          border: token("info-border"),
        },
        destructive: {
          DEFAULT: token("destructive"),
          soft: token("destructive-soft"),
          border: token("destructive-border"),
        },
      },
      fontFamily: {
        sans: ['"Geist Sans"', ...defaultTheme.fontFamily.sans],
        mono: ['"Geist Mono"', ...defaultTheme.fontFamily.mono],
      },
      boxShadow: {
        // Cool-tinted elevation, keyed off --shadow-color so dark mode reads
        // as depth rather than a gray halo.
        card: "0 1px 2px -1px rgb(var(--shadow-color) / 0.08), 0 2px 8px -2px rgb(var(--shadow-color) / 0.06)",
        elevated:
          "0 4px 12px -2px rgb(var(--shadow-color) / 0.12), 0 2px 6px -2px rgb(var(--shadow-color) / 0.08)",
      },
    },
  },
  plugins: [],
} satisfies Config;
