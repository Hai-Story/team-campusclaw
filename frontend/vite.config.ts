import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发时代理路径与生产 Nginx 保持一致：同源 /api（design.md D1）。
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8081', changeOrigin: false },
      '/health': { target: 'http://localhost:8081', changeOrigin: false },
    },
  },
})
