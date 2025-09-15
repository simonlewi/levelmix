/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./core/templates/**/*.html",
    "./core/templates/pages/*.html",
    "./core/static/js/**/*.js",
  ],
    safelist: [
    'fixed',
    'inset-0',
    'z-50',
    'bg-black',
    'bg-opacity-75',
    'min-h-screen'
  ],
  theme: {
    extend: {
      colors: {
        primary: '#3864F5',
        secondary: '#6B46FE', 
        accent: '#FF4791',
      }
    },
  },
}