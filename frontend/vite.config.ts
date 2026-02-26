import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    // WebView2 is always a modern Chromium; target esnext to skip
    // unnecessary syntax down-levelling and keep the bundles lean.
    target: 'esnext',
    rollupOptions: {
      output: {
        // Keep the heavy maplibre-gl in its own chunk so WebView2 can
        // cache it independently and parse it separately from app code.
        manualChunks: {
          maplibre: ['maplibre-gl'],
        },
      },
    },
  },
})
