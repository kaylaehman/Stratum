/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        'bg-base': 'var(--bg-base)',
        'bg-surface': 'var(--bg-surface)',
        'bg-elevated': 'var(--bg-elevated)',
        'bg-overlay': 'var(--bg-overlay)',
        'border-subtle': 'var(--border-subtle)',
        'border-default': 'var(--border-default)',
        'border-strong': 'var(--border-strong)',
        'text-primary': 'var(--text-primary)',
        'text-secondary': 'var(--text-secondary)',
        'text-muted': 'var(--text-muted)',
        'text-inverse': 'var(--text-inverse)',
        accent: 'var(--accent)',
        'accent-dim': 'var(--accent-dim)',
        'status-ok': 'var(--status-ok)',
        'status-warn': 'var(--status-warn)',
        'status-error': 'var(--status-error)',
        'status-info': 'var(--status-info)',
        'status-muted': 'var(--status-muted)',
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'monospace'],
      },
      fontSize: {
        base: ['0.875rem', { lineHeight: '1.5' }],
      },
      borderRadius: {
        btn: '3px',
      },
    },
  },
  plugins: [],
}
