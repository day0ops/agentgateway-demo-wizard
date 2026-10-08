import { PrismLight as SyntaxHighlighter } from "react-syntax-highlighter";
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";
import json from "react-syntax-highlighter/dist/esm/languages/prism/json";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

SyntaxHighlighter.registerLanguage("yaml", yaml);
SyntaxHighlighter.registerLanguage("json", json);

export function CodeBlock({
  code,
  language = "yaml",
}: {
  code: string;
  language?: "yaml" | "json";
}) {
  return (
    <SyntaxHighlighter
      language={language}
      style={vscDarkPlus}
      wrapLongLines
      className="rounded bg-slate-950 text-xs"
      customStyle={{ background: "transparent", margin: 0, padding: "0.75rem" }}
      codeTagProps={{ style: { fontFamily: "inherit" } }}
    >
      {code}
    </SyntaxHighlighter>
  );
}
