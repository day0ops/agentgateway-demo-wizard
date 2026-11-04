// Shared control styles, so every pane's buttons and inputs resolve to the
// same tokens instead of each re-declaring (and drifting on) its own Tailwind
// strings. Compose with extra classes via a template string where needed.

// Filled teal action - the one primary call-to-action per pane.
export const btnPrimary =
  "inline-flex items-center justify-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground shadow-sm transition-all duration-150 ease-out hover:bg-primary-hover active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50";

// Neutral filled action - secondary choices (Revert, Log out, presets).
export const btnSecondary =
  "inline-flex items-center justify-center gap-1.5 rounded-lg border border-border bg-secondary px-3 py-1.5 text-sm font-medium text-secondary-foreground transition-all duration-150 ease-out hover:bg-secondary-hover active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50";

// Small quiet toggle/utility chip (view YAML, Rotate, Send test request).
export const btnChip =
  "inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-2.5 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground active:scale-[0.97] disabled:pointer-events-none disabled:opacity-50";

// Text inputs and textareas.
export const inputBase =
  "rounded-lg border border-input bg-card px-3 py-1.5 text-sm text-foreground placeholder:text-subtle transition-colors focus-visible:border-primary";
