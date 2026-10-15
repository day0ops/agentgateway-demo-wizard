// Shared react-markdown component overrides, so every markdown-rendered
// surface in the app (agent answers, step explanations) gets the same
// typography instead of each caller reinventing its own list/link styling.
export const markdownComponents = {
  a: ({ ...props }) => (
    <a
      {...props}
      target="_blank"
      rel="noreferrer"
      className="text-brand-600 underline dark:text-brand-400"
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
      className="rounded bg-slate-100 px-1 py-0.5 text-xs dark:bg-zinc-800"
    />
  ),
};
