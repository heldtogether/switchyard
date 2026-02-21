import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"] ,
  theme: {
    extend: {
      colors: {
        ink: {
          900: "#0c1116",
          800: "#111823",
          700: "#1a2230",
          600: "#273043",
          500: "#3a455a",
          400: "#56617a",
          300: "#78839b",
          200: "#a7b0c2",
          100: "#cfd4df",
          50: "#eef0f5"
        },
        brand: {
          600: "#2b6cb0",
          500: "#3b82f6",
          400: "#60a5fa"
        },
        success: "#16a34a",
        danger: "#dc2626",
        warning: "#f59e0b",
        info: "#2563eb",
        pending: "#6b7280"
      },
      fontFamily: {
        sans: ["Sora", "ui-sans-serif", "system-ui"],
        mono: ["JetBrains Mono", "ui-monospace", "SFMono-Regular"]
      },
      boxShadow: {
        card: "0 12px 40px rgba(15, 23, 42, 0.08)",
        float: "0 8px 20px rgba(15, 23, 42, 0.12)"
      },
      borderRadius: {
        xl: "1rem"
      }
    }
  },
  plugins: []
} satisfies Config;
