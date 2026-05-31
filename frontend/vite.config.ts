import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
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
      workbox: {
        // Precache the compiled app shell and static assets only.
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff,woff2}'],

        runtimeCaching: [
          {
            // Google Fonts stylesheets — stale-while-revalidate is safe.
            urlPattern: /^https:\/\/fonts\.googleapis\.com\/.*/i,
            handler: 'StaleWhileRevalidate',
            options: {
              cacheName: 'google-fonts-stylesheets',
              expiration: { maxEntries: 4, maxAgeSeconds: 60 * 60 * 24 * 365 },
            },
          },
          {
            // Google Fonts files — cache-first (versioned/immutable URLs).
            urlPattern: /^https:\/\/fonts\.gstatic\.com\/.*/i,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-webfonts',
              expiration: { maxEntries: 10, maxAgeSeconds: 60 * 60 * 24 * 365 },
            },
          },
          {
            // /api/* and /health — NEVER cache.
            // This rule also covers /api/ws so the SW never intercepts
            // WebSocket upgrades or SSE log/metric streams.
            urlPattern: /^\/(api|health)(\/|$)/,
            handler: 'NetworkOnly',
          },
        ],

        // Offline navigation fallback: the app shell for any non-API path.
        navigateFallback: '/index.html',
        // Exclude API routes and health endpoint from fallback entirely.
        navigateFallbackDenylist: [/^\/(api|health)(\/|$)/],

        // Activate the new SW immediately but don't skipWaiting automatically;
        // we call it explicitly from the update prompt in main.tsx.
        clientsClaim: true,
        skipWaiting: false,
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
