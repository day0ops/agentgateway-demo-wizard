import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { mockApi } from "./dev-fixtures/mock-api";

// MOCK_API=1 serves the demo backend from local fixtures (no cluster needed),
// for design/preview work. Unset, dev proxies /api to the real Go server.
const useMock = process.env.MOCK_API === "1";

export default defineConfig({
  plugins: [react(), ...(useMock ? [mockApi()] : [])],
  server: useMock
    ? {}
    : {
        proxy: {
          "/api": "http://localhost:8080",
        },
      },
  build: {
    outDir: "dist",
  },
});
