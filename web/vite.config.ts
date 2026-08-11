import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import katex from 'katex'

// The odds equations are static, so KaTeX runs here at build time and only its
// HTML output ships — the browser never loads the katex runtime.
const tex = (s: string) => katex.renderToString(s, { displayMode: true, throwOnError: false })

const oddsMath = {
  chance: tex(String.raw`P = \min\!\left(100,\; \left\lfloor \tfrac{\mathrm{base}(s)}{r+1} \right\rfloor + 2c\right)`),
  roll: tex(String.raw`\text{success} \iff U < \min(100,\ P + T),\qquad U \sim \mathcal{U}\{0,\dots,99\}`),
  gain: tex(String.raw`\mathrm{gain} = \begin{cases} 1 & r = 0 \\ \max\!\bigl(1,\ \operatorname{round}(100/P)\bigr) & r \ge 1 \end{cases}`),
  next: tex(String.raw`s' = \min\!\bigl(s + \mathrm{gain}\cdot D,\ \mathrm{cap}\bigr),\qquad \mathrm{cap} = 15 + 5\min(p,3),\qquad D = \begin{cases} 2 & \text{prismatic talisman} \\ 1 & \text{else} \end{cases}`),
  jackpot: tex(String.raw`\text{jackpot} = 5\,\max\!\bigl(0,\ s + \mathrm{gain}\cdot D - \mathrm{cap}\bigr)\ 💰`),
  fail: tex(String.raw`s' = \begin{cases} s & \text{holo talisman or }🛡 \\ \min(\mathrm{headstart},\, s) & \text{else} \end{cases}`),
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  define: {
    __ODDS_MATH__: JSON.stringify(oddsMath),
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
