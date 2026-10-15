import { useEffect, useState } from "react";
import { PrismLight as SyntaxHighlighter } from "react-syntax-highlighter";
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";
import json from "react-syntax-highlighter/dist/esm/languages/prism/json";
import {
  vs,
  vscDarkPlus,
} from "react-syntax-highlighter/dist/esm/styles/prism";
import { getTheme, onThemeChange } from "../lib/theme";

SyntaxHighlighter.registerLanguage("yaml", yaml);
SyntaxHighlighter.registerLanguage("json", json);

export function CodeBlock({
  code,
  language = "yaml",
}: {
  code: string;
  language?: "yaml" | "json";
}) {
  const [theme, setThemeState] = useState(getTheme);
  useEffect(() => onThemeChange(() => setThemeState(getTheme())), []);

  return (
    <SyntaxHighlighter
      language={language}
      style={theme === "dark" ? vscDarkPlus : vs}
      wrapLongLines
      className="rounded bg-slate-50 text-xs dark:bg-slate-950"
      customStyle={{ background: "transparent", margin: 0, padding: "0.75rem" }}
      codeTagProps={{ style: { fontFamily: "inherit" } }}
    >
      {code}
    </SyntaxHighlighter>
  );
}
