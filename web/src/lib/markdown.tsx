// Shared react-markdown component overrides, so every markdown-rendered
// surface in the app (agent answers, step explanations) gets the same
// typography instead of each caller reinventing its own list/link styling.
export const markdownComponents = {
  a: ({ ...props }) => (
    <a
      {...props}
      target="_blank"
      rel="noreferrer"
      className="font-medium text-primary underline decoration-primary/40 underline-offset-2 transition-colors hover:decoration-primary"
    />
  ),
  ul: ({ ...props }) => <ul {...props} className="list-disc space-y-1 pl-5" />,
  ol: ({ ...props }) => (
    <ol {...props} className="list-decimal space-y-1 pl-5" />
  ),
  p: ({ ...props }) => <p {...props} className="[&:not(:first-child)]:mt-2" />,
  code: ({ ...props }) => (
    <code
      {...props}
      className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[0.85em] text-foreground"
    />
  ),
};
