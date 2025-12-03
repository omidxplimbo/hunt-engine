/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        hack: {
          bg: '#050505',
          panel: '#0a0a0a',
          border: '#333333',
          primary: '#00ff41', // سبز ماتریکسی
          secondary: '#008F11',
          danger: '#ff003c',
          warning: '#fcee0a',
          text: '#e0e0e0',
          dim: '#666666',
        }
      },
      fontFamily: {
        mono: ['"Fira Code"', '"JetBrains Mono"', '"Courier New"', 'monospace'],
        display: ['"VT323"', 'monospace'],
      },
      boxShadow: {
        'neon': '0 0 5px rgba(0, 255, 65, 0.2), 0 0 10px rgba(0, 255, 65, 0.1)',
        'neon-red': '0 0 5px rgba(255, 0, 60, 0.2), 0 0 10px rgba(255, 0, 60, 0.1)',
      },
      backgroundImage: {
        'grid-pattern': "linear-gradient(to right, #111 1px, transparent 1px), linear-gradient(to bottom, #111 1px, transparent 1px)",
      }
    },
  },
  plugins: [],
}