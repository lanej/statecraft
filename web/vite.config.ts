import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => ({
  server: {
    proxy: {
      "/runtime":
        loadEnv(mode, ".", "STATECRAFT_").STATECRAFT_API_URL ||
        "http://127.0.0.1:8081",
      "/statecraft.v1.":
        loadEnv(mode, ".", "STATECRAFT_").STATECRAFT_API_URL ||
        "http://127.0.0.1:8081",
    },
  },
}));
