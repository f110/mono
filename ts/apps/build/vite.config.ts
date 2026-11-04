import tanstackRouter from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const bffURL = 'http://127.0.0.1:8082'

export default defineConfig(({ mode }) => {
  return {
    server: {
      host: '127.0.0.1',
      port: 8301,
      proxy: {
        '/mono.build.bff.BFF': {
          target: bffURL,
        },
      },
    },
    plugins: [
      tanstackRouter({
        target: 'react',
        autoCodeSplitting: true,
      }),
      react(),
    ],
    build: {
      outDir: 'dist_' + mode,
    },
  }
})
