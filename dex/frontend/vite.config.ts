import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": "/src" } },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8088",
      "/ws": { target: "ws://127.0.0.1:8088", ws: true },
    },
  },
  build: {
    target: "es2022",
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: { chakra: ["@chakra-ui/react"], charts: ["lightweight-charts"] },
      },
    },
  },
});
