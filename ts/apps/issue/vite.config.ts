import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const apiURL = 'http://127.0.0.1:8095'

export default defineConfig(({ mode }) => {
  return {
    server: {
      host: '127.0.0.1',
      port: 8302,
      proxy: {
        '/bff': {
          target: apiURL,
          rewrite: (path) => path.replace(/^\/bff/, ''),
        },
      },
    },
    plugins: [
      react(),
    ],
    build: {
      outDir: 'dist_' + mode,
    },
  }
})
