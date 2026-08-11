/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Enterprise navy — primary brand + dark surfaces
        navy: {
          50: '#eef3fb',
          100: '#d8e3f5',
          200: '#b3c8ea',
          300: '#86a4d9',
          400: '#4f78c2',
          500: '#2d56a3',
          600: '#1f4286',
          700: '#17356c',
          800: '#122b58',
          900: '#0F2D5A',
          950: '#0a1f40',
        },
        // Emerald — success / positive money
        emerald: {
          50: '#ecfdf5',
          100: '#d1fae5',
          200: '#a7f3d0',
          300: '#6ee7b7',
          400: '#34d399',
          500: '#10B981',
          600: '#059669',
          700: '#047857',
          800: '#065f46',
          900: '#064e3b',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      backgroundImage: {
        // Paper-like grid at full opacity — the user's design system
        'paper-grid':
          'linear-gradient(to right, rgba(15,45,90,0.05) 1px, transparent 1px), linear-gradient(to bottom, rgba(15,45,90,0.05) 1px, transparent 1px)',
      },
      backgroundSize: {
        grid: '24px 24px',
      },
    },
  },
  plugins: [],
}
