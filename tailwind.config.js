// Build with `make css`. Vendor libraries are left out of `content` on purpose:
// scanning them would only add classes the app never uses.
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './web/index.html',
    './web/js/app.js',
    './web/js/{components,data,store,views}/**/*.js',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: { sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'] },
      colors: {
        ink: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#14213d',
          950: '#0f172a',
        },
        brand: {
          50: '#eff6ff',
          100: '#dbeafe',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
        },
        money: {
          pos: '#16a34a',
          neg: '#dc2626',
        },
      },
    },
  },
}
