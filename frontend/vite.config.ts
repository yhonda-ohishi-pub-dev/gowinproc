import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': '/src',
    },
  },
  server: {
    host: '127.0.0.1',
    port: 3000,
    strictPort: true, // ポート3000が使用中の場合はエラーを出す（多重起動防止）
    hmr: {
      // HMR (Hot Module Replacement) 設定
      // リロード時にサーバーとの接続を維持
      overlay: true, // エラーオーバーレイを表示
      clientPort: 3000,
    },
    // Keep-Alive設定でバックエンドとの接続を維持
    cors: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        secure: false,
        // Keep-Alive設定でバックエンドとの接続を維持
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, _req, _res) => {
            proxyReq.setHeader('Connection', 'keep-alive')
          })
        },
      },
      '/health': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        secure: false,
      },
    },
  },
})