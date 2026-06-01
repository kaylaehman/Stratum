import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      // injectManifest: use our custom src/sw.ts so we can add push + notificationclick
      // handlers on top of the existing Workbox precaching strategy.
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',

      registerType: 'prompt',
      // Manual SW registration in main.tsx gives us control over the
      // update-available prompt (skipWaiting only fires after user confirms).
      injectRegister: null,
      manifest: {
        name: 'Stratum',
        short_name: 'Stratum',
        description:
          'Unified infrastructure management — hosts, VMs, containers, filesystems.',
        theme_color: '#0b0e13',
        background_color: '#0b0e13',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        lang: 'en',
        icons: [
          {
            src: '/icon-192.png',
            sizes: '192x192',
            type: 'image/png',
          },
          {
            src: '/icon-512.png',
            sizes: '512x512',
            type: 'image/png',
          },
          {
            src: '/icon-maskable-512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
      // injectManifest options: passed to workbox-build's injectManifest().
      injectManifest: {
        // Precache the compiled app shell and static assets only.
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff,woff2}'],
      },
    }),
  ],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
