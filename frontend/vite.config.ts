import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// L'application est servie depuis l'exécutable, pas depuis un serveur : les
// chemins doivent être relatifs et tout doit tenir dans le bundle.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 900,
  },
  server: { port: 5173, strictPort: true },
})
