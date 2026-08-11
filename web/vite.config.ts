import { defineConfig, loadEnv, type Plugin } from 'vite'
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

// robots.txt / sitemap.xml / security.txt all need the canonical origin, and
// Vite only substitutes %VITE_*% inside index.html — so they're emitted here
// instead, keeping VITE_SITE_URL the single source of truth.
const botFiles = (origin: string, contact: string): Plugin => ({
  name: 'bot-files',
  generateBundle() {
    const emit = (fileName: string, source: string) =>
      this.emitFile({ type: 'asset', fileName, source })

    emit('robots.txt', `User-agent: *\nAllow: /\nDisallow: /api/\n\nSitemap: ${origin}/sitemap.xml\n`)
    emit(
      'sitemap.xml',
      `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n  <url>\n    <loc>${origin}/</loc>\n    <changefreq>daily</changefreq>\n    <priority>1.0</priority>\n  </url>\n</urlset>\n`,
    )
    // Re-stamped on every build, so the expiry can't quietly go stale.
    const expires = new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString()
    emit(
      '.well-known/security.txt',
      `Contact: ${contact}\nExpires: ${expires}\nPreferred-Languages: ko, en\nCanonical: ${origin}/.well-known/security.txt\n`,
    )
  },
})

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_')
  if (!env.VITE_SITE_URL || !env.VITE_SECURITY_CONTACT) {
    throw new Error('VITE_SITE_URL and VITE_SECURITY_CONTACT must be set (see web/.env)')
  }
  const origin = env.VITE_SITE_URL.replace(/\/$/, '')

  return {
    plugins: [vue(), tailwindcss(), botFiles(origin, env.VITE_SECURITY_CONTACT)],
    define: {
      __ODDS_MATH__: JSON.stringify(oddsMath),
    },
    server: {
      proxy: {
        '/api': 'http://localhost:8080',
      },
    },
  }
})
