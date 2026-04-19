/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#fef7ee',
          100: '#fdedd6',
          500: '#f97316',
          600: '#ea6b0a',
          700: '#c2530a',
        },
      },
    },
  },
  plugins: [],
}
