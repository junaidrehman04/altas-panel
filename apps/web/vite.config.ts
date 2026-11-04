import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

const proxy = {
  "/api": {
    target: "http://127.0.0.1:8080",
    changeOrigin: true,
  },
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: "0.0.0.0",
    port: 5173,
    strictPort: true,
    proxy,
  },
  preview: {
    host: "0.0.0.0",
    port: 5173,
    strictPort: true,
    proxy,
  },
})
