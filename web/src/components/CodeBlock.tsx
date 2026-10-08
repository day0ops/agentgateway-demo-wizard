import { PrismLight as SyntaxHighlighter } from "react-syntax-highlighter";
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

SyntaxHighlighter.registerLanguage("yaml", yaml);

export function CodeBlock({ code }: { code: string }) {
  return (
    <SyntaxHighlighter
      language="yaml"
      style={vscDarkPlus}
      className="overflow-x-auto rounded bg-slate-950 text-xs"
      customStyle={{ background: "transparent", margin: 0, padding: "0.75rem" }}
      codeTagProps={{ style: { fontFamily: "inherit" } }}
    >
      {code}
    </SyntaxHighlighter>
  );
}
