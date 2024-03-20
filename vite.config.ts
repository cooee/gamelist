import react from "@vitejs/plugin-react"
import path from "path"
import { defineConfig } from "vite"

export default defineConfig({
  base: process.env.NODE_ENV === "development" ? "/" : "/test/" , // 默认 '/'
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build:{
    outDir: 'dist', // 默认是 'dist'  
  }
})