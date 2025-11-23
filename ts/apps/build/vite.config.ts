import tanstackRouter from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import * as path from 'node:path'
import { defineConfig } from 'vite'

const envDir = path.join(__dirname, './env')

export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: 8301,
  },
  envDir,
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
    }),
    react(),
  ],
})
