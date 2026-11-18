import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const apiURL = 'http://127.0.0.1:7480'

export default defineConfig(({ mode }) => {
  return {
    server: {
      host: '127.0.0.1',
      port: 8303,
      proxy: {
        '/mono.codexel': {
          target: apiURL,
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
